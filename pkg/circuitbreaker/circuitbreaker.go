package circuitbreaker

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed - circuit breaker is closed, requests flow normally
	StateClosed State = iota
	// StateOpen - circuit breaker is open, requests are rejected
	StateOpen
	// StateHalfOpen - circuit breaker is half-open, testing if service is recovered
	StateHalfOpen
)

// String returns string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config holds circuit breaker configuration
type Config struct {
	// Name of the circuit breaker for logging/metrics
	Name string

	// MaxRequests is the maximum number of requests allowed to pass through
	// when the circuit breaker is half-open
	MaxRequests uint32

	// Interval is the cyclic period of the closed state
	// for the circuit breaker to clear the internal counts
	Interval time.Duration

	// Timeout is the period of the open state,
	// after which the state of the circuit breaker becomes half-open
	Timeout time.Duration

	// ReadyToTrip returns true when the circuit breaker should trip
	// from closed to open state
	ReadyToTrip func(counts Counts) bool

	// OnStateChange is called whenever the state of the circuit breaker changes
	OnStateChange func(name string, from State, to State)

	// IsSuccessful returns true if the error should be considered a success
	// By default, nil error is considered success
	IsSuccessful func(err error) bool
}

// Counts holds the numbers of requests and their successes/failures
type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

// CircuitBreaker represents a circuit breaker
type CircuitBreaker struct {
	name          string
	maxRequests   uint32
	interval      time.Duration
	timeout       time.Duration
	readyToTrip   func(counts Counts) bool
	onStateChange func(name string, from State, to State)
	isSuccessful  func(err error) bool

	mutex      sync.Mutex
	state      State
	generation uint64
	counts     Counts
	expiry     time.Time

	logger *logrus.Logger
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config Config, logger *logrus.Logger) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:        config.Name,
		maxRequests: config.MaxRequests,
		interval:    config.Interval,
		timeout:     config.Timeout,
		logger:      logger,
	}

	// Set default values
	if cb.maxRequests == 0 {
		cb.maxRequests = 1
	}
	if cb.interval <= 0 {
		cb.interval = 60 * time.Second
	}
	if cb.timeout <= 0 {
		cb.timeout = 60 * time.Second
	}

	// Set callback functions
	if config.ReadyToTrip != nil {
		cb.readyToTrip = config.ReadyToTrip
	} else {
		cb.readyToTrip = defaultReadyToTrip
	}

	if config.OnStateChange != nil {
		cb.onStateChange = config.OnStateChange
	} else {
		cb.onStateChange = cb.defaultOnStateChange
	}

	if config.IsSuccessful != nil {
		cb.isSuccessful = config.IsSuccessful
	} else {
		cb.isSuccessful = defaultIsSuccessful
	}

	cb.toNewGeneration(time.Now())

	return cb
}

// Execute executes the given function if the circuit breaker accepts it
func (cb *CircuitBreaker) Execute(fn func() error) error {
	generation, err := cb.beforeRequest()
	if err != nil {
		return err
	}

	defer func() {
		e := recover()
		if e != nil {
			cb.afterRequest(generation, false)
			panic(e)
		}
	}()

	result := fn()
	cb.afterRequest(generation, cb.isSuccessful(result))
	return result
}

// Call is an alias for Execute for better readability
func (cb *CircuitBreaker) Call(fn func() error) error {
	return cb.Execute(fn)
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, _ := cb.currentState(now)
	return state
}

// Counts returns a copy of the current counts
func (cb *CircuitBreaker) Counts() Counts {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	return cb.counts
}

// Name returns the name of the circuit breaker
func (cb *CircuitBreaker) Name() string {
	return cb.name
}

// beforeRequest is called before a request
func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)

	if state == StateOpen {
		return generation, ErrOpenState
	} else if state == StateHalfOpen && cb.counts.Requests >= cb.maxRequests {
		return generation, ErrTooManyRequests
	}

	cb.counts.Requests++
	return generation, nil
}

// afterRequest is called after a request
func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	now := time.Now()
	state, generation := cb.currentState(now)
	if generation != before {
		return
	}

	if success {
		cb.onSuccess(state, now)
	} else {
		cb.onFailure(state, now)
	}
}

// onSuccess handles successful requests
func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
	cb.counts.TotalSuccesses++
	cb.counts.ConsecutiveSuccesses++
	cb.counts.ConsecutiveFailures = 0

	if state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= cb.maxRequests {
		cb.setState(StateClosed, now)
	}
}

// onFailure handles failed requests
func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
	cb.counts.TotalFailures++
	cb.counts.ConsecutiveFailures++
	cb.counts.ConsecutiveSuccesses = 0

	if state == StateClosed {
		if cb.readyToTrip(cb.counts) {
			cb.setState(StateOpen, now)
		}
	} else if state == StateHalfOpen {
		cb.setState(StateOpen, now)
	}
}

// currentState returns the current state and generation
func (cb *CircuitBreaker) currentState(now time.Time) (State, uint64) {
	switch cb.state {
	case StateClosed:
		if !cb.expiry.IsZero() && cb.expiry.Before(now) {
			cb.toNewGeneration(now)
		}
	case StateOpen:
		if cb.expiry.Before(now) {
			cb.setState(StateHalfOpen, now)
		}
	}
	return cb.state, cb.generation
}

// setState changes the state of the circuit breaker
func (cb *CircuitBreaker) setState(state State, now time.Time) {
	if cb.state == state {
		return
	}

	prev := cb.state
	cb.state = state

	cb.toNewGeneration(now)

	if cb.onStateChange != nil {
		cb.onStateChange(cb.name, prev, state)
	}
}

// toNewGeneration creates a new generation
func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
	cb.generation++
	cb.counts = Counts{}

	var zero time.Time
	switch cb.state {
	case StateClosed:
		if cb.interval == 0 {
			cb.expiry = zero
		} else {
			cb.expiry = now.Add(cb.interval)
		}
	case StateOpen:
		cb.expiry = now.Add(cb.timeout)
	default: // StateHalfOpen
		cb.expiry = zero
	}
}

// defaultOnStateChange is the default state change callback
func (cb *CircuitBreaker) defaultOnStateChange(name string, from State, to State) {
	cb.logger.WithFields(logrus.Fields{
		"circuit_breaker": name,
		"from_state":      from.String(),
		"to_state":        to.String(),
	}).Warn("Circuit breaker state changed")
}

// Predefined errors
var (
	ErrOpenState        = errors.New("circuit breaker is open")
	ErrTooManyRequests  = errors.New("too many requests")
	ErrInvalidThreshold = errors.New("invalid threshold")
)

// defaultReadyToTrip is the default function to determine if the circuit should trip
func defaultReadyToTrip(counts Counts) bool {
	return counts.Requests >= 5 && counts.ConsecutiveFailures >= 5
}

// defaultIsSuccessful is the default function to determine if a request was successful
func defaultIsSuccessful(err error) bool {
	return err == nil
}

// Settings for common circuit breaker configurations

// DefaultConfig returns a default circuit breaker configuration
func DefaultConfig(name string) Config {
	return Config{
		Name:        name,
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 3 && failureRatio >= 0.6
		},
	}
}

// AggressiveConfig returns an aggressive circuit breaker configuration
// that trips quickly and recovers slowly
func AggressiveConfig(name string) Config {
	return Config{
		Name:        name,
		MaxRequests: 1,
		Interval:    30 * time.Second,
		Timeout:     120 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}
}

// ConservativeConfig returns a conservative circuit breaker configuration
// that is slower to trip and faster to recover
func ConservativeConfig(name string) Config {
	return Config{
		Name:        name,
		MaxRequests: 5,
		Interval:    120 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 10 && failureRatio >= 0.8
		},
	}
}

// WebhookConfig returns a circuit breaker configuration optimized for webhook calls
func WebhookConfig(name string) Config {
	return Config{
		Name:        name,
		MaxRequests: 2,
		Interval:    60 * time.Second,
		Timeout:     90 * time.Second,
		ReadyToTrip: func(counts Counts) bool {
			// Trip if we have at least 3 requests and 70% failure rate
			// or 5 consecutive failures
			if counts.Requests >= 3 {
				failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
				return failureRatio >= 0.7 || counts.ConsecutiveFailures >= 5
			}
			return counts.ConsecutiveFailures >= 5
		},
		IsSuccessful: func(err error) bool {
			if err == nil {
				return true
			}
			// Consider timeout and connection errors as failures
			// but not HTTP 4xx errors (client errors)
			// TODO: Implement more sophisticated error classification
			return false // For now, consider all errors as failures
		},
	}
}

// Manager manages multiple circuit breakers
type Manager struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
	logger   *logrus.Logger
}

// NewManager creates a new circuit breaker manager
func NewManager(logger *logrus.Logger) *Manager {
	return &Manager{
		breakers: make(map[string]*CircuitBreaker),
		logger:   logger,
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (m *Manager) GetOrCreate(name string, config Config) *CircuitBreaker {
	m.mutex.RLock()
	if cb, exists := m.breakers[name]; exists {
		m.mutex.RUnlock()
		return cb
	}
	m.mutex.RUnlock()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Double-check after acquiring write lock
	if cb, exists := m.breakers[name]; exists {
		return cb
	}

	config.Name = name
	cb := NewCircuitBreaker(config, m.logger)
	m.breakers[name] = cb
	return cb
}

// Get gets an existing circuit breaker
func (m *Manager) Get(name string) (*CircuitBreaker, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	cb, exists := m.breakers[name]
	return cb, exists
}

// Remove removes a circuit breaker
func (m *Manager) Remove(name string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.breakers, name)
}

// List returns all circuit breaker names
func (m *Manager) List() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.breakers))
	for name := range m.breakers {
		names = append(names, name)
	}
	return names
}

// Stats returns statistics for all circuit breakers
func (m *Manager) Stats() map[string]CircuitBreakerStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for name, cb := range m.breakers {
		stats[name] = CircuitBreakerStats{
			Name:   name,
			State:  cb.State(),
			Counts: cb.Counts(),
		}
	}
	return stats
}

// CircuitBreakerStats represents circuit breaker statistics
type CircuitBreakerStats struct {
	Name   string `json:"name"`
	State  State  `json:"state"`
	Counts Counts `json:"counts"`
}

// String returns a string representation of the stats
func (s CircuitBreakerStats) String() string {
	return fmt.Sprintf("CircuitBreaker[%s]: State=%s, Requests=%d, Successes=%d, Failures=%d",
		s.Name, s.State.String(), s.Counts.Requests, s.Counts.TotalSuccesses, s.Counts.TotalFailures)
}
