# Dcard Assignment - Resource Pool

## Thoughts
- I prefer to use struct pointers for interface response, since it allows for null values and less room for callers to misunderstand response. I followed the interface provided, and used zero value of the structs instead when returning errors.
- When I started working on this assignment, I spent some time to think how the resource pool would be used. These eventually became the unit tests.
- This assignment, I wanted to focus on clear and readable code. If there are any questions, I am happy to explain further.