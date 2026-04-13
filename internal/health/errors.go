package health

import "errors"

// errCircuitOpen is returned when the circuit breaker is open and rejecting requests.
// Exported via export_test.go for test assertions.
var errCircuitOpen = errors.New("health: circuit breaker is open")
