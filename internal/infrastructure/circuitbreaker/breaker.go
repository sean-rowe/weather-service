package circuitbreaker

import (
    "context"
    "errors"
    "sync"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.uber.org/zap"
)

var (
    ErrCircuitOpen     = errors.New("circuit breaker is open")
    ErrTooManyRequests = errors.New("too many requests")
)

type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

func (s State) String() string {
    switch s {
    case StateClosed:
        return "closed"
    case StateOpen:
        return "open"
    case StateHalfOpen:
        return "half-open"
    default:
        return "unknown"
    }
}

type Config struct {
    MaxRequests     uint32
    Interval        time.Duration
    Timeout         time.Duration
    FailureRatio    float64
    MinimumRequests uint32
}

type CircuitBreaker struct {
    config Config
    logger *zap.Logger
    
    mu              sync.RWMutex
    state           State
    generation      uint64
    counts          *Counts
    expiry          time.Time
    lastStateChange time.Time
}

type Counts struct {
    Requests             uint32
    TotalSuccesses       uint32
    TotalFailures        uint32
    ConsecutiveSuccesses uint32
    ConsecutiveFailures  uint32
}

func NewCircuitBreaker(config Config, logger *zap.Logger) *CircuitBreaker {
    return &CircuitBreaker{
        config:          config,
        logger:          logger,
        state:           StateClosed,
        counts:          &Counts{},
        lastStateChange: time.Now(),
    }
}

func (cb *CircuitBreaker) Execute(ctx context.Context, name string, fn func() error) error {
    tracer := otel.Tracer("circuit-breaker")
    ctx, span := tracer.Start(ctx, "CircuitBreaker.Execute")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("circuit_breaker.name", name),
        attribute.String("circuit_breaker.state", cb.State().String()),
    )
    
    generation, err := cb.beforeRequest()
    if err != nil {
        span.SetAttributes(attribute.Bool("circuit_breaker.rejected", true))
        cb.logger.Warn("circuit breaker rejected request",
            zap.String("name", name),
            zap.String("state", cb.State().String()),
            zap.Error(err),
        )
        return err
    }
    
    defer func() {
        e := recover()
        if e != nil {
            cb.afterRequest(generation, false)
            panic(e)
        }
    }()
    
    err = fn()
    success := err == nil
    cb.afterRequest(generation, success)
    
    span.SetAttributes(
        attribute.Bool("circuit_breaker.success", success),
        attribute.String("circuit_breaker.new_state", cb.State().String()),
    )
    
    return err
}

func (cb *CircuitBreaker) beforeRequest() (uint64, error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    now := time.Now()
    state, generation := cb.currentState(now)
    
    if state == StateOpen {
        return generation, ErrCircuitOpen
    }
    
    if state == StateHalfOpen && cb.counts.Requests >= cb.config.MaxRequests {
        return generation, ErrTooManyRequests
    }
    
    cb.counts.Requests++
    return generation, nil
}

func (cb *CircuitBreaker) afterRequest(before uint64, success bool) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
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

func (cb *CircuitBreaker) onSuccess(state State, now time.Time) {
    switch state {
    case StateClosed:
        cb.counts.TotalSuccesses++
        cb.counts.ConsecutiveSuccesses++
        cb.counts.ConsecutiveFailures = 0
        
    case StateHalfOpen:
        cb.counts.TotalSuccesses++
        cb.counts.ConsecutiveSuccesses++
        
        if cb.counts.ConsecutiveSuccesses >= cb.config.MaxRequests {
            cb.setState(StateClosed, now)
        }
    }
}

func (cb *CircuitBreaker) onFailure(state State, now time.Time) {
    switch state {
    case StateClosed:
        cb.counts.TotalFailures++
        cb.counts.ConsecutiveFailures++
        cb.counts.ConsecutiveSuccesses = 0
        
        if cb.shouldTrip() {
            cb.setState(StateOpen, now)
        }
        
    case StateHalfOpen:
        cb.setState(StateOpen, now)
    }
}

func (cb *CircuitBreaker) shouldTrip() bool {
    if cb.counts.Requests < cb.config.MinimumRequests {
        return false
    }
    
    failureRatio := float64(cb.counts.TotalFailures) / float64(cb.counts.Requests)
    return failureRatio >= cb.config.FailureRatio
}

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

func (cb *CircuitBreaker) setState(state State, now time.Time) {
    if cb.state == state {
        return
    }
    
    prev := cb.state
    cb.state = state
    cb.lastStateChange = now
    
    cb.toNewGeneration(now)
    
    if state == StateOpen {
        cb.expiry = now.Add(cb.config.Timeout)
    } else {
        cb.expiry = time.Time{}
    }
    
    cb.logger.Info("circuit breaker state changed",
        zap.String("from", prev.String()),
        zap.String("to", state.String()),
        zap.Time("at", now),
    )
}

func (cb *CircuitBreaker) toNewGeneration(now time.Time) {
    cb.generation++
    cb.counts = &Counts{}
    
    switch cb.state {
    case StateClosed:
        if cb.config.Interval > 0 {
            cb.expiry = now.Add(cb.config.Interval)
        }
    case StateOpen:
        cb.expiry = now.Add(cb.config.Timeout)
    default:
        cb.expiry = time.Time{}
    }
}

func (cb *CircuitBreaker) State() State {
    cb.mu.RLock()
    defer cb.mu.RUnlock()
    
    now := time.Now()
    state, _ := cb.currentState(now)
    return state
}

func (cb *CircuitBreaker) Stats() Stats {
    cb.mu.RLock()
    defer cb.mu.RUnlock()
    
    return Stats{
        State:                cb.state.String(),
        Requests:             cb.counts.Requests,
        TotalSuccesses:       cb.counts.TotalSuccesses,
        TotalFailures:        cb.counts.TotalFailures,
        ConsecutiveSuccesses: cb.counts.ConsecutiveSuccesses,
        ConsecutiveFailures:  cb.counts.ConsecutiveFailures,
        LastStateChange:      cb.lastStateChange,
    }
}

type Stats struct {
    State                string    `json:"state"`
    Requests             uint32    `json:"requests"`
    TotalSuccesses       uint32    `json:"total_successes"`
    TotalFailures        uint32    `json:"total_failures"`
    ConsecutiveSuccesses uint32    `json:"consecutive_successes"`
    ConsecutiveFailures  uint32    `json:"consecutive_failures"`
    LastStateChange      time.Time `json:"last_state_change"`
}

type Manager struct {
    breakers map[string]*CircuitBreaker
    mu       sync.RWMutex
    logger   *zap.Logger
}

func NewManager(logger *zap.Logger) *Manager {
    return &Manager{
        breakers: make(map[string]*CircuitBreaker),
        logger:   logger,
    }
}

func (m *Manager) GetBreaker(name string, config Config) *CircuitBreaker {
    m.mu.RLock()
    if breaker, exists := m.breakers[name]; exists {
        m.mu.RUnlock()
        return breaker
    }
    m.mu.RUnlock()
    
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if breaker, exists := m.breakers[name]; exists {
        return breaker
    }
    
    breaker := NewCircuitBreaker(config, m.logger.With(zap.String("circuit_breaker", name)))
    m.breakers[name] = breaker
    
    return breaker
}

func (m *Manager) GetStats() map[string]Stats {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    stats := make(map[string]Stats)
    for name, breaker := range m.breakers {
        stats[name] = breaker.Stats()
    }
    
    return stats
}