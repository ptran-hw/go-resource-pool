package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

var _ Pool[PoolResource] = &NewPool[PoolResource]{}

type Pool[T any] interface {
	Acquire(context.Context) (T, error)
	Release(T)
	NumIdle() int
}

type NewPool[T comparable] struct {
	creator     func(ctx context.Context) (T, error)
	maxIdleSize int
	maxIdleTime time.Duration
	mutex       PoolMutex
	lock        map[T]time.Time
	unlock      map[T]time.Time
}

type PoolResource struct {
}

type PoolMutex interface {
	Lock()
	Unlock()
}

// creates or returns a ready-to-use item from the resource pool
func (n NewPool[T]) Acquire(ctx context.Context) (T, error) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	n.deleteInvalidIdleResources()

	if resource, isSuccess := n.getIdleResource(); isSuccess {
		return resource, nil
	}

	// creates resource
	resource, err := n.creator(ctx)
	if err != nil {
		return *new(T), err
	}

	n.lock[resource] = time.Now()
	return resource, nil
}

// releases an active resource back to the resource pool
func (n NewPool[T]) Release(resource T) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	savedTimestamp, isFound := n.lock[resource]
	if !isFound {
		fmt.Println("resource not previously acquired; not returning to idle resource pool")
		return
	}

	delete(n.lock, resource)

	validTimestamp := n.getValidTimestamp()
	if savedTimestamp.Before(validTimestamp) {
		fmt.Println("resource already expired; not returning to idle resource pool")
		return
	}
	if n.NumIdle() >= n.maxIdleSize {
		fmt.Println("resource already expired; not returning to idle resource pool")
		return
	}

	n.unlock[resource] = time.Now()
}

// returns the number of idle items
func (n NewPool[T]) NumIdle() int {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	return len(n.unlock)
}

// cleans up expired idle resources
func (n NewPool[T]) deleteInvalidIdleResources() {
	validTimestamp := n.getValidTimestamp()

	for key, savedTimestamp := range n.unlock {
		if savedTimestamp.Before(validTimestamp) {
			delete(n.unlock, key)
		}
	}
}

// retrieves idle resource
func (n NewPool[T]) getIdleResource() (T, bool) {
	for resource, _ := range n.unlock {
		delete(n.unlock, resource)
		n.lock[resource] = time.Now()
		return resource, true
	}

	return *new(T), false
}

func (n NewPool[T]) getValidTimestamp() time.Time {
	return time.Now().Add(-1 * n.maxIdleTime)
}

func New[T comparable](
	// creator is a function called by the pool to create a resource.
	creator func(context.Context) (T, error),
	// maxIdleSize is the number of maximum idle items kept in the pool
	maxIdleSize int,
	// maxIdleTime is the maximum idle time for an idle item to be swept from the pool
	maxIdleTime time.Duration,
) Pool[T] {
	return &NewPool[T]{
		creator:     creator,
		maxIdleSize: maxIdleSize,
		maxIdleTime: maxIdleTime,
		mutex:       &sync.Mutex{},
		lock:        make(map[T]time.Time),
		unlock:      make(map[T]time.Time),
	}
}
