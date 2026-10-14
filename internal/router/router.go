// Package router implements the toolset registry and on-demand loading.
//
// The starter kit (~2K token budget) is always registered.
// Additional toolsets are registered on demand via enable_toolset.
package router

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Toolset groups related tools.
type Toolset struct {
	Name        string
	Description string
	// Adder is called once when the toolset is enabled; it calls mcp.AddTool for each tool.
	Adder func(s *mcp.Server, deps *Deps)
}

// Deps is the shared dependency bundle passed to every tool handler.
type Deps struct {
	Sessions *SessionRegistry
	Jobs     interface{ List() interface{} } // jobmgr.Manager — use interface to avoid import cycle
	Log      interface {
		Record(interface{})
		Recent(int, string) interface{}
		Stats() interface{}
	}
}

// Registry manages toolsets.
type Registry struct {
	mu      sync.Mutex
	server  *mcp.Server
	deps    *Deps
	all     map[string]*Toolset
	enabled map[string]bool
}

// New creates a Registry.
func New(server *mcp.Server, deps *Deps) *Registry {
	return &Registry{
		server:  server,
		deps:    deps,
		all:     make(map[string]*Toolset),
		enabled: make(map[string]bool),
	}
}

// Register adds a toolset (does not enable it).
func (r *Registry) Register(ts *Toolset) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.all[ts.Name] = ts
}

// Enable activates a toolset, adding its tools to the server.
// Returns an error if the name is unknown or already enabled.
func (r *Registry) Enable(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ts, ok := r.all[name]
	if !ok {
		return fmt.Errorf("unknown toolset %q; use list_toolsets to see available toolsets", name)
	}
	if r.enabled[name] {
		return fmt.Errorf("toolset %q is already enabled", name)
	}
	ts.Adder(r.server, r.deps)
	r.enabled[name] = true
	return nil
}

// List returns descriptions of all registered toolsets.
func (r *Registry) List() []ToolsetInfo {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ToolsetInfo, 0, len(r.all))
	for name, ts := range r.all {
		out = append(out, ToolsetInfo{
			Name:        name,
			Description: ts.Description,
			Enabled:     r.enabled[name],
		})
	}
	return out
}

// ToolsetInfo describes a toolset.
type ToolsetInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

// SessionRegistry manages named Vivado sessions.
type SessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]interface{} // name -> *session.Session (interface to avoid import cycle)
}

// NewSessionRegistry creates an empty registry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{sessions: make(map[string]interface{})}
}

// Register registers a session.
func (sr *SessionRegistry) Register(name string, s interface{}) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.sessions[name] = s
}

// Get retrieves a session by name.
func (sr *SessionRegistry) Get(name string) (interface{}, bool) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	s, ok := sr.sessions[name]
	return s, ok
}

// Names returns all registered session names.
func (sr *SessionRegistry) Names() []string {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	names := make([]string, 0, len(sr.sessions))
	for n := range sr.sessions {
		names = append(names, n)
	}
	return names
}

// Delete removes a session from the registry.
func (sr *SessionRegistry) Delete(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.sessions, name)
}

// TextResult is a helper that returns a plain-text MCP tool result.
func TextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// ErrorResult returns an MCP tool result with IsError=true.
func ErrorResult(text string) *mcp.CallToolResult {
	t := true
	return &mcp.CallToolResult{
		IsError: &t,
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

// NoOp satisfies the compiler for unused context.
var _ = context.Background
