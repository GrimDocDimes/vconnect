// Package router implements the toolset registry and on-demand loading.
//
// The starter kit (~2K token budget) is always registered.
// Additional toolsets are registered on demand via enable_toolset.
package router

import (
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
// Currently unused (tools capture Env directly via closures), but kept as an
// extension point for future toolsets.
type Deps struct{}

// Registry manages toolsets.
type Registry struct {
	mu      sync.Mutex
	server  *mcp.Server
	all     map[string]*Toolset
	enabled map[string]bool
}

// New creates a Registry.
func New(server *mcp.Server, _ *Deps) *Registry {
	return &Registry{
		server:  server,
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
	ts.Adder(r.server, nil)
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
