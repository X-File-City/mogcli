package graph

import (
	"sync"
	"time"
)

const (
	CircuitBreakerThreshold = 5
	CircuitBreakerResetTime = 30 * time.Second
)

type CircuitBreaker struct {
	mu          sync.Mutex
	failures    int
	lastFailure time.Time
	open        bool
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{}
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.open = false
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= CircuitBreakerThreshold {
		cb.open = true
	}
}

func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.isOpenLocked(time.Now())
}

// Execute atomically checks whether the breaker is open, runs fn, and then
// records success/failure state.
//
// fn returns (recordFailure, err):
//   - err == nil: records success and resets failures.
//   - err != nil, recordFailure == true: records a breaker failure.
//   - err != nil, recordFailure == false: returns err without changing breaker state.
func (cb *CircuitBreaker) Execute(fn func() (bool, error)) error {
	if fn == nil {
		return nil
	}

	cb.mu.Lock()
	if cb.isOpenLocked(time.Now()) {
		cb.mu.Unlock()
		return &CircuitBreakerError{}
	}
	cb.mu.Unlock()

	recordFailure, err := fn()
	if err == nil {
		cb.RecordSuccess()
		return nil
	}
	if !recordFailure {
		return err
	}

	cb.RecordFailure()
	return err
}

func (cb *CircuitBreaker) isOpenLocked(now time.Time) bool {
	if !cb.open {
		return false
	}

	if now.Sub(cb.lastFailure) > CircuitBreakerResetTime {
		cb.open = false
		cb.failures = 0
		return false
	}

	return true
}
