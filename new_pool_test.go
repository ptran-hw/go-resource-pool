package main

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
	"time"
)

const maxIdleSize = 3
const maxIdleTime = 5 * time.Second

type MockResource struct {
	id int
}

type MockMutex struct {
	mock.Mock
}

func (m *MockMutex) Lock() {
	m.Called()
}

func (m *MockMutex) Unlock() {
	m.Called()
}

func TestNewPool_Acquire(t *testing.T) {
	testCases := []struct {
		name                   string
		creator                func(ctx context.Context) (MockResource, error)
		idleResourcePool       map[MockResource]time.Time
		expectedResource       MockResource
		expectedError          error
		expectedUsedPoolLength int
		expectedIdlePoolLength int
	}{
		{
			name: "with expired idle resource updates idle resource pool",
			idleResourcePool: map[MockResource]time.Time{
				MockResource{
					id: 2,
				}: time.Now().Add(-2 * maxIdleTime),
			},
			expectedUsedPoolLength: 1,
			expectedIdlePoolLength: 0,
		},
		{
			name: "with non-empty idle resource pool returns existing resource",
			idleResourcePool: map[MockResource]time.Time{
				MockResource{
					id: 2,
				}: time.Now(),
			},
			expectedResource: MockResource{
				id: 2,
			},
			expectedUsedPoolLength: 1,
			expectedIdlePoolLength: 0,
		},
		{
			name:             "with empty idle resource pool returns new resource",
			idleResourcePool: map[MockResource]time.Time{},
			expectedResource: MockResource{
				id: 1,
			},
			expectedUsedPoolLength: 1,
			expectedIdlePoolLength: 0,
		},
		{
			name:             "with creator func error response returns error",
			creator:          getErrorMockCreatorFunc(),
			idleResourcePool: map[MockResource]time.Time{},
			expectedResource: MockResource{
				id: 1,
			},
			expectedUsedPoolLength: 1,
			expectedIdlePoolLength: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.creator == nil {
				tc.creator = getMockCreatorFunc()
			}
			if tc.idleResourcePool == nil {
				tc.idleResourcePool = make(map[MockResource]time.Time)
			}

			mockMutex := &MockMutex{}
			mockMutex.On("Lock")
			mockMutex.On("Unlock")

			pool := NewPool[MockResource]{
				creator:     getMockCreatorFunc(),
				maxIdleTime: maxIdleTime,
				maxIdleSize: maxIdleSize,
				mutex:       mockMutex,
				unlock:      tc.idleResourcePool,
				lock:        make(map[MockResource]time.Time),
			}

			resource, err := pool.Acquire(nil)

			if tc.expectedResource != *new(MockResource) {
				assert.Equal(t, tc.expectedResource, resource)
			}
			assert.Equal(t, tc.expectedError, err)

			assert.Equal(t, tc.expectedUsedPoolLength, len(pool.lock))
			assert.Equal(t, tc.expectedIdlePoolLength, len(pool.unlock))
			mockMutex.AssertExpectations(t)
		})
	}
}

func TestNewPool_Release(t *testing.T) {
	testCases := []struct {
		name                   string
		resource               MockResource
		usedResourcePool       map[MockResource]time.Time
		idleResourcePool       map[MockResource]time.Time
		expectedUsedPoolLength int
		expectedIdlePoolLength int
	}{
		{
			name:     "with non-acquired resource does not update idle pool",
			resource: MockResource{id: 2},
			usedResourcePool: map[MockResource]time.Time{
				MockResource{
					id: 1,
				}: time.Now(),
			},
			expectedUsedPoolLength: 1,
			expectedIdlePoolLength: 0,
		},
		{
			name:     "with expired resource does not update idle pool",
			resource: MockResource{id: 2},
			usedResourcePool: map[MockResource]time.Time{
				MockResource{
					id: 2,
				}: time.Now().Add(-2 * maxIdleTime),
			},
			expectedUsedPoolLength: 0,
			expectedIdlePoolLength: 0,
		},
		{
			name:     "with valid resource updates idle pool",
			resource: MockResource{id: 2},
			usedResourcePool: map[MockResource]time.Time{
				MockResource{
					id: 2,
				}: time.Now(),
			},
			expectedUsedPoolLength: 0,
			expectedIdlePoolLength: 1,
		},
		{
			name:     "with valid resource and full idle pool does not update idle pool",
			resource: MockResource{id: 2},
			usedResourcePool: map[MockResource]time.Time{
				MockResource{
					id: 2,
				}: time.Now(),
			},
			idleResourcePool: map[MockResource]time.Time{
				MockResource{
					id: 5,
				}: time.Now(),
				MockResource{
					id: 6,
				}: time.Now(),
				MockResource{
					id: 7,
				}: time.Now(),
			},
			expectedUsedPoolLength: 0,
			expectedIdlePoolLength: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.idleResourcePool == nil {
				tc.idleResourcePool = make(map[MockResource]time.Time)
			}

			mockMutex := &MockMutex{}
			mockMutex.On("Lock")
			mockMutex.On("Unlock")

			pool := NewPool[MockResource]{
				creator:     getMockCreatorFunc(),
				maxIdleTime: maxIdleTime,
				maxIdleSize: maxIdleSize,
				mutex:       mockMutex,
				lock:        tc.usedResourcePool,
				unlock:      tc.idleResourcePool,
			}

			pool.Release(tc.resource)

			assert.Equal(t, tc.usedResourcePool, pool.lock)
			assert.Equal(t, tc.expectedUsedPoolLength, len(pool.lock))
			assert.Equal(t, tc.expectedIdlePoolLength, len(pool.unlock))
			mockMutex.AssertExpectations(t)
		})
	}
}

func TestNewPool_NumIdle(t *testing.T) {
	testCases := []struct {
		name           string
		idlePoolCount  int
		expectedLength int
	}{
		{
			name:           "with empty idle pool returns 0",
			idlePoolCount:  0,
			expectedLength: 0,
		},
		{
			name:           "with non-empty pool returns > 0",
			idlePoolCount:  3,
			expectedLength: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockMutex := &MockMutex{}
			mockMutex.On("Lock")
			mockMutex.On("Unlock")

			unlock := make(map[MockResource]time.Time)
			for i := 1; i <= tc.idlePoolCount; i++ {
				unlock[MockResource{id: i}] = time.Now()
			}

			pool := NewPool[MockResource]{
				creator:     getMockCreatorFunc(),
				maxIdleTime: maxIdleTime,
				maxIdleSize: maxIdleSize,
				mutex:       mockMutex,
				lock:        make(map[MockResource]time.Time),
				unlock:      unlock,
			}

			assert.Equal(t, tc.expectedLength, pool.NumIdle())
			mockMutex.AssertExpectations(t)
		})
	}
}

func getMockCreatorFunc() func(context.Context) (MockResource, error) {
	id := 0
	return func(ctx context.Context) (MockResource, error) {
		id += 1
		return MockResource{id}, nil
	}
}

func getErrorMockCreatorFunc() func(context.Context) (MockResource, error) {
	return func(ctx context.Context) (MockResource, error) {
		return *new(MockResource), errors.New("error response")
	}
}
