package graph

import (
	"errors"
	"testing"
)

func TestCircuitBreakerExecuteReturnsOpenErrorWithoutRunningFn(t *testing.T) {
	cb := NewCircuitBreaker()
	for i := 0; i < CircuitBreakerThreshold; i++ {
		cb.RecordFailure()
	}

	ran := false
	err := cb.Execute(func() (bool, error) {
		ran = true
		return false, nil
	})
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}

	var breakerErr *CircuitBreakerError
	if !errors.As(err, &breakerErr) {
		t.Fatalf("expected CircuitBreakerError, got %T (%v)", err, err)
	}
	if ran {
		t.Fatal("expected callback to be skipped while breaker is open")
	}
}

func TestCircuitBreakerExecuteRecordsFailureWhenRequested(t *testing.T) {
	cb := NewCircuitBreaker()

	testErr := errors.New("server unavailable")
	err := cb.Execute(func() (bool, error) {
		return true, testErr
	})
	if !errors.Is(err, testErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if cb.failures != 1 {
		t.Fatalf("expected one failure recorded, got %d", cb.failures)
	}
}

func TestCircuitBreakerExecuteSkipsFailureRecordWhenNotRequested(t *testing.T) {
	cb := NewCircuitBreaker()

	testErr := errors.New("bad request")
	err := cb.Execute(func() (bool, error) {
		return false, testErr
	})
	if !errors.Is(err, testErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
	if cb.failures != 0 {
		t.Fatalf("expected no failures recorded, got %d", cb.failures)
	}
}

func TestCircuitBreakerExecuteResetsOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.RecordFailure()
	cb.RecordFailure()

	if err := cb.Execute(func() (bool, error) {
		return false, nil
	}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if cb.failures != 0 {
		t.Fatalf("expected failures reset to zero, got %d", cb.failures)
	}
	if cb.open {
		t.Fatal("expected breaker to be closed after success")
	}
}
