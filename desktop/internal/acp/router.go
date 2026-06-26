package acp

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type Router struct {
	mu          sync.RWMutex
	adapters    map[string]ACPAdapter
	lastUsed    map[string]time.Time
	idleTimeout time.Duration
}

func NewRouter() *Router {
	r := &Router{
		adapters:    make(map[string]ACPAdapter),
		lastUsed:    make(map[string]time.Time),
		idleTimeout: 30 * time.Minute,
	}
	go r.idleReaper()
	return r
}

func (r *Router) Register(adapter ACPAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	info := adapter.Info()
	r.adapters[info.ID] = adapter
}

func (r *Router) Route(agentID, prompt string, history []Message) (<-chan StreamEvent, error) {
	r.mu.RLock()
	adapter, ok := r.adapters[agentID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown agent: %s", agentID)
	}
	r.mu.Lock()
	r.lastUsed[agentID] = time.Now()
	r.mu.Unlock()
	return adapter.Send(prompt, history)
}

func (r *Router) LoadSession(agentID, acpSessionID string, history []Message) error {
	r.mu.RLock()
	adapter, ok := r.adapters[agentID]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown agent: %s", agentID)
	}
	r.mu.Lock()
	r.lastUsed[agentID] = time.Now()
	r.mu.Unlock()
	return adapter.LoadSession(acpSessionID, history)
}

func (r *Router) CurrentSessionID(agentID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.adapters[agentID]
	if !ok {
		return "", fmt.Errorf("unknown agent: %s", agentID)
	}
	return adapter.CurrentSessionID(), nil
}

func (r *Router) ListAgents() []AgentInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	infos := make([]AgentInfo, 0, len(r.adapters))
	for _, a := range r.adapters {
		infos = append(infos, a.Info())
	}
	return infos
}

func (r *Router) GetAgent(id string) (AgentInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[id]
	if !ok {
		return AgentInfo{}, false
	}
	return a.Info(), true
}

func (r *Router) idleReaper() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		for id, last := range r.lastUsed {
			if time.Since(last) > r.idleTimeout {
				if adapter, ok := r.adapters[id]; ok && adapter.IsRunning() {
					log.Printf("[router] idle timeout for %s, stopping", id)
					adapter.Stop()
				}
				delete(r.lastUsed, id)
			}
		}
		r.mu.Unlock()
	}
}
