package kairos

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Evaluator orchestrates policy evaluation with caching and fallback behavior
type Evaluator struct {
	client *Client
	config *PolicyConfig
	logger *zap.Logger

	// Cache: key is hash of (trace_id, signal_id), value is (decision, expiry)
	cacheMu    sync.RWMutex
	decisionCache map[string]*cachedDecision
	maxCacheSize  int
	cacheTTL      time.Duration

	// Metrics
	metricsLock sync.RWMutex
	evaluations int64
	cacheHits   int64
	cacheMisses int64
	errors      int64
}

type cachedDecision struct {
	decision *PolicyDecision
	expiry   time.Time
}

// NewEvaluator creates a new Kairos evaluator with client and caching
func NewEvaluator(client *Client, config *PolicyConfig, logger *zap.Logger) *Evaluator {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}

	return &Evaluator{
		client:        client,
		config:        config,
		logger:        logger,
		decisionCache: make(map[string]*cachedDecision),
		maxCacheSize:  config.CacheSize,
		cacheTTL:      time.Duration(config.CacheTTLSec) * time.Second,
	}
}

// Evaluate runs policy evaluation with caching and fail-open fallback.
// Returns (*PolicyDecision, error). On timeout or unreachable, returns (nil, err)
// so callers can distinguish timeout from disabled/cached-nil.
func (e *Evaluator) Evaluate(ctx context.Context, req *EvaluationRequest) (*PolicyDecision, error) {
	if !e.config.Enabled {
		// If Kairos is disabled, return nil (no policy decision)
		return nil, nil
	}

	// Increment evaluation counter
	e.metricsLock.Lock()
	e.evaluations++
	e.metricsLock.Unlock()

	// Cache key includes rule_id to prevent cross-rule contamination (Pitfall 5).
	cacheKey := fmt.Sprintf("%s:%s:%s", req.RuleID, req.SignalID, req.TraceID)
	if cached := e.getCachedDecision(cacheKey); cached != nil {
		e.metricsLock.Lock()
		e.cacheHits++
		e.metricsLock.Unlock()

		e.logger.Debug("decision cache hit",
			zap.String("signal_id", req.SignalID),
			zap.String("decision", cached.Decision))
		return cached, nil
	}

	e.metricsLock.Lock()
	e.cacheMisses++
	e.metricsLock.Unlock()

	// Call Kairos (with timeout already in client)
	decision, err := e.client.Evaluate(ctx, req)
	if err != nil {
		// Only log errors if not a timeout/unreachable (those are expected for fail-open)
		e.logger.Warn("kairos evaluation error",
			zap.String("signal_id", req.SignalID),
			zap.Error(err))

		e.metricsLock.Lock()
		e.errors++
		e.metricsLock.Unlock()

		// Return err so callers can distinguish timeout vs nil decision.
		return nil, err
	}

	// If Kairos returned nil (unreachable), that's also fail-open
	if decision == nil {
		return nil, nil
	}

	// Cache the decision
	e.cacheDecision(cacheKey, decision)

	e.logger.Debug("kairos evaluation succeeded",
		zap.String("signal_id", req.SignalID),
		zap.String("decision", decision.Decision),
		zap.Float64("confidence", decision.Confidence))

	return decision, nil
}

// getCachedDecision retrieves a decision from cache if valid
func (e *Evaluator) getCachedDecision(key string) *PolicyDecision {
	e.cacheMu.RLock()
	defer e.cacheMu.RUnlock()

	cached, ok := e.decisionCache[key]
	if !ok {
		return nil
	}

	// Check if expired
	if time.Now().After(cached.expiry) {
		return nil
	}

	return cached.decision
}

// cacheDecision stores a decision in the cache
func (e *Evaluator) cacheDecision(key string, decision *PolicyDecision) {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()

	// Simple eviction: if cache is full, clear oldest entries (not LRU, but good enough)
	if len(e.decisionCache) >= e.maxCacheSize {
		// Clear 20% of entries (simple eviction strategy)
		count := 0
		for k := range e.decisionCache {
			delete(e.decisionCache, k)
			count++
			if count >= e.maxCacheSize/5 {
				break
			}
		}
	}

	e.decisionCache[key] = &cachedDecision{
		decision: decision,
		expiry:   time.Now().Add(e.cacheTTL),
	}
}

// ClearCache clears the decision cache
func (e *Evaluator) ClearCache() {
	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()
	e.decisionCache = make(map[string]*cachedDecision)
}

// Metrics returns evaluation metrics
func (e *Evaluator) Metrics() map[string]interface{} {
	e.metricsLock.RLock()
	defer e.metricsLock.RUnlock()

	cacheSize := 0
	e.cacheMu.RLock()
	cacheSize = len(e.decisionCache)
	e.cacheMu.RUnlock()

	return map[string]interface{}{
		"evaluations":  e.evaluations,
		"cache_hits":   e.cacheHits,
		"cache_misses": e.cacheMisses,
		"cache_size":   cacheSize,
		"errors":       e.errors,
	}
}

// ResetMetrics clears accumulated metrics
func (e *Evaluator) ResetMetrics() {
	e.metricsLock.Lock()
	defer e.metricsLock.Unlock()
	e.evaluations = 0
	e.cacheHits = 0
	e.cacheMisses = 0
	e.errors = 0
}
