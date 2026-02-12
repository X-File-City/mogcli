package graph

import "fmt"

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return fmt.Sprintf("Graph API %d (%s): %s", e.Status, e.Code, e.Message)
	}

	return fmt.Sprintf("Graph API %d: %s", e.Status, e.Message)
}

type CircuitBreakerError struct{}

func (e *CircuitBreakerError) Error() string {
	return "graph client circuit breaker open"
}
