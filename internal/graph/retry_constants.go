package graph

import "time"

const (
	MaxRateLimitRetries   = 4
	RateLimitBaseDelay    = 1 * time.Second
	Max5xxRetries         = 2
	ServerErrorRetryDelay = 1 * time.Second
)
