package engine

import "sync"

// RuleStore is an in-memory registry of detection rules.
type RuleStore struct {
	mu    sync.RWMutex
	rules map[string]Rule
}

// NewRuleStore creates an empty in-memory rule store.
func NewRuleStore() *RuleStore {
	return &RuleStore{rules: map[string]Rule{}}
}

// Add creates or replaces a rule by ID.
func (s *RuleStore) Add(rule Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules[rule.ID] = rule
}

// Remove deletes a rule by ID.
func (s *RuleStore) Remove(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rules, id)
}

// Get returns a rule by ID.
func (s *RuleStore) Get(id string) (Rule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rules[id]
	return r, ok
}

// All returns all rules in undefined order.
func (s *RuleStore) All() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Rule, 0, len(s.rules))
	for _, r := range s.rules {
		out = append(out, r)
	}
	return out
}

// Enabled returns only enabled rules.
func (s *RuleStore) Enabled() []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Rule, 0, len(s.rules))
	for _, r := range s.rules {
		if r.Enabled {
			out = append(out, r)
		}
	}
	return out
}

// Count returns number of rules.
func (s *RuleStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.rules)
}

// ReplaceAll atomically replaces all rules.
func (s *RuleStore) ReplaceAll(rules []Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make(map[string]Rule, len(rules))
	for _, r := range rules {
		next[r.ID] = r
	}
	s.rules = next
}
