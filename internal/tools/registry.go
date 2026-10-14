// session registry for tools package – wraps session.Session to keep
// the tools package import-clean.
package tools

import (
	"sync"
	"github.com/jhush/vconnect/internal/session"
)

// Registry manages named Vivado sessions.
type Registry struct {
	mu   sync.Mutex
	sess map[string]*session.Session
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{sess: make(map[string]*session.Session)}
}

// Register adds a session under name.
func (r *Registry) Register(name string, s *session.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sess[name] = s
}

// Get retrieves a session by name.
func (r *Registry) Get(name string) (*session.Session, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.sess[name]
	return s, ok
}

// Delete removes a session.
func (r *Registry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sess, name)
}

// All returns all sessions.
func (r *Registry) All() []*session.Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*session.Session, 0, len(r.sess))
	for _, s := range r.sess {
		out = append(out, s)
	}
	return out
}

// Names returns all registered session names.
func (r *Registry) Names() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	names := make([]string, 0, len(r.sess))
	for n := range r.sess {
		names = append(names, n)
	}
	return names
}
