// Package tools implements all MCP tool handlers for vconnect.
//
// Starter kit tools (always registered):
//   server_stats, get_recent_calls, list_toolsets, enable_toolset,
//   session_start, session_status, run_tcl, tcl_help
//
// On-demand toolsets: target, project, flow, reports, constraints, ip, bd,
//   checkpoints, hardware.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/jhush/vconnect/internal/jobmgr"
	"github.com/jhush/vconnect/internal/observability"
	"github.com/jhush/vconnect/internal/router"
	"github.com/jhush/vconnect/internal/session"
)

// Env bundles all server-wide singletons available to every handler.
type Env struct {
	Sessions *Registry
	Jobs     *jobmgr.Manager
	Log      *observability.Logger
	Registry *router.Registry
	Config   *Config
}

// Config holds server startup configuration.
type Config struct {
	VivadoBin    string
	BootstrapTcl string
	TmpDir       string
	RunsDir      string
	LogPath      string
}

// RegisterStarterKit registers the always-available tools.
func RegisterStarterKit(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "server_stats",
		Description: "Return server uptime, Vivado version, call counts per tool, active sessions, active jobs.",
	}, serverStats(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_recent_calls",
		Description: "Return the N most recent tool calls, optionally filtered by tool name.",
	}, getRecentCalls(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_toolsets",
		Description: "List all available toolsets and whether they are currently enabled.",
	}, listToolsets(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "enable_toolset",
		Description: "Enable a toolset by name, adding its tools. Valid names: target, project, flow, reports, constraints, ip, bd, checkpoints, hardware.",
	}, enableToolset(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_start",
		Description: "Start a named Vivado session. Name defaults to 'main'. Returns session ID and Vivado version.",
	}, sessionStart(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "session_status",
		Description: "Return the status (idle/busy/crashed/starting) and Vivado version for one or all sessions.",
	}, sessionStatus(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "run_tcl",
		Description: "Execute arbitrary Tcl in a Vivado session. Returns stdout value, Vivado messages, and duration. Use for commands not covered by typed tools. Script runs under [catch]; errors are returned structured, never silently swallowed.",
	}, runTcl(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "tcl_help",
		Description: "Run 'help <command>' or 'help -category <cat>' inside Vivado and return the output. Never invent Tcl commands — use this to verify them.",
	}, tclHelp(env))
}

// ── Tool input/output types ────────────────────────────────────────────────

type serverStatsIn struct{}
type serverStatsOut struct {
	Uptime    string                  `json:"uptime"`
	StartedAt time.Time               `json:"started_at"`
	Sessions  []sessionStatusEntry    `json:"sessions"`
	Jobs      []jobmgr.StatusFile     `json:"active_jobs"`
	Tools     []observability.ToolStat `json:"tool_stats"`
}

type getRecentCallsIn struct {
	N      int    `json:"n"       jsonschema:"number of calls to return, default 20"`
	Filter string `json:"filter"  jsonschema:"optional tool name filter"`
}

type listToolsetsIn struct{}

type enableToolsetIn struct {
	Name string `json:"name" jsonschema:"toolset name: target|project|flow|reports|constraints|ip|bd|checkpoints|hardware"`
}

type sessionStartIn struct {
	Name      string `json:"name"       jsonschema:"session name, default 'main'"`
	VivadoBin string `json:"vivado_bin" jsonschema:"override path to vivado binary"`
}
type sessionStartOut struct {
	Name          string `json:"name"`
	VivadoVersion string `json:"vivado_version"`
	State         string `json:"state"`
}

type sessionStatusIn struct {
	Name string `json:"name" jsonschema:"session name, or empty for all sessions"`
}
type sessionStatusEntry struct {
	Name          string `json:"name"`
	State         string `json:"state"`
	StartedAt     string `json:"started_at"`
	VivadoVersion string `json:"vivado_version,omitempty"`
}

type runTclIn struct {
	Script  string `json:"script"          jsonschema:"Tcl script to execute"`
	Session string `json:"session"         jsonschema:"session name, default 'main'"`
	Timeout int    `json:"timeout_seconds" jsonschema:"per-call timeout in seconds, default 300"`
}

type tclHelpIn struct {
	Command string `json:"command"  jsonschema:"Vivado Tcl command or category to look up"`
	Session string `json:"session"  jsonschema:"session name, default 'main'"`
}

// ── Handler implementations ────────────────────────────────────────────────

func serverStats(env *Env) mcp.ToolHandlerFor[serverStatsIn, serverStatsOut] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in serverStatsIn) (*mcp.CallToolResult, serverStatsOut, error) {
		stats := env.Log.Stats()
		sessions := env.Sessions.All() // []*session.Session
		var sessionEntries []sessionStatusEntry
		for _, s := range sessions {
			sessionEntries = append(sessionEntries, sessionStatusEntry{
				Name:      s.Name(),
				State:     s.GetState().String(),
				StartedAt: s.StartedAt().Format(time.RFC3339),
			})
		}

		allJobs := env.Jobs.List()
		var active []jobmgr.StatusFile
		for _, j := range allJobs {
			if j.Status == jobmgr.JobRunning || j.Status == jobmgr.JobPending {
				active = append(active, j)
			}
		}

		out := serverStatsOut{
			Uptime:    stats.Uptime.Round(time.Second).String(),
			StartedAt: stats.StartedAt,
			Sessions:  sessionEntries,
			Jobs:      active,
			Tools:     stats.Tools,
		}
		return nil, out, nil
	}
}

func getRecentCalls(env *Env) mcp.ToolHandlerFor[getRecentCallsIn, []observability.CallEntry] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getRecentCallsIn) (*mcp.CallToolResult, []observability.CallEntry, error) {
		n := in.N
		if n <= 0 {
			n = 20
		}
		entries := env.Log.Recent(n, in.Filter)
		return nil, entries, nil
	}
}

func listToolsets(env *Env) mcp.ToolHandlerFor[listToolsetsIn, []router.ToolsetInfo] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in listToolsetsIn) (*mcp.CallToolResult, []router.ToolsetInfo, error) {
		return nil, env.Registry.List(), nil
	}
}

func enableToolset(env *Env) mcp.ToolHandlerFor[enableToolsetIn, router.ToolsetInfo] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in enableToolsetIn) (*mcp.CallToolResult, router.ToolsetInfo, error) {
		if in.Name == "" {
			return nil, router.ToolsetInfo{}, fmt.Errorf("name is required")
		}
		if err := env.Registry.Enable(in.Name); err != nil {
			return nil, router.ToolsetInfo{}, err
		}
		for _, ts := range env.Registry.List() {
			if ts.Name == in.Name {
				return nil, ts, nil
			}
		}
		return nil, router.ToolsetInfo{Name: in.Name, Enabled: true}, nil
	}
}

func sessionStart(env *Env) mcp.ToolHandlerFor[sessionStartIn, sessionStartOut] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in sessionStartIn) (*mcp.CallToolResult, sessionStartOut, error) {
		name := in.Name
		if name == "" {
			name = "main"
		}
		if _, exists := env.Sessions.Get(name); exists {
			return nil, sessionStartOut{}, fmt.Errorf("session %q already exists; use session_status to check it", name)
		}

		bin := in.VivadoBin
		if bin == "" {
			bin = env.Config.VivadoBin
		}

		cfg := session.Config{
			VivadoBin:    bin,
			BootstrapTcl: env.Config.BootstrapTcl,
			TmpDir:       env.Config.TmpDir,
			Name:         name,
		}
		s, err := session.New(ctx, cfg)
		if err != nil {
			return nil, sessionStartOut{}, fmt.Errorf("failed to start Vivado session %q: %w", name, err)
		}
		env.Sessions.Register(name, s)

		ver, _ := s.VivadoVersion(ctx)
		return nil, sessionStartOut{
			Name:          name,
			VivadoVersion: ver,
			State:         s.GetState().String(),
		}, nil
	}
}

func sessionStatus(env *Env) mcp.ToolHandlerFor[sessionStatusIn, []sessionStatusEntry] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in sessionStatusIn) (*mcp.CallToolResult, []sessionStatusEntry, error) {
		var out []sessionStatusEntry
		if in.Name != "" {
			s, ok := env.Sessions.Get(in.Name)
			if !ok {
				return nil, nil, fmt.Errorf("session %q not found", in.Name)
			}
			out = append(out, sessionStatusEntry{
				Name:      s.Name(),
				State:     s.GetState().String(),
				StartedAt: s.StartedAt().Format(time.RFC3339),
			})
		} else {
			for _, s := range env.Sessions.All() {
				out = append(out, sessionStatusEntry{
					Name:      s.Name(),
					State:     s.GetState().String(),
					StartedAt: s.StartedAt().Format(time.RFC3339),
				})
			}
		}
		return nil, out, nil
	}
}

func runTcl(env *Env) mcp.ToolHandlerFor[runTclIn, *session.ExecResult] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in runTclIn) (*mcp.CallToolResult, *session.ExecResult, error) {
		if in.Script == "" {
			return nil, nil, fmt.Errorf("script is required")
		}
		sessName := in.Session
		if sessName == "" {
			sessName = "main"
		}
		s, ok := env.Sessions.Get(sessName)
		if !ok {
			return nil, nil, fmt.Errorf("session %q not found; call session_start first", sessName)
		}
		timeout := time.Duration(in.Timeout) * time.Second
		res, err := s.Exec(ctx, in.Script, timeout)
		if err != nil {
			return nil, nil, err
		}
		return nil, res, nil
	}
}

func tclHelp(env *Env) mcp.ToolHandlerFor[tclHelpIn, *session.ExecResult] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in tclHelpIn) (*mcp.CallToolResult, *session.ExecResult, error) {
		if in.Command == "" {
			return nil, nil, fmt.Errorf("command is required")
		}
		sessName := in.Session
		if sessName == "" {
			sessName = "main"
		}
		s, ok := env.Sessions.Get(sessName)
		if !ok {
			return nil, nil, fmt.Errorf("session %q not found; call session_start first", sessName)
		}
		// Use Vivado's built-in help
		script := fmt.Sprintf("help %s", in.Command)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, nil, err
		}
		return nil, res, nil
	}
}

// ── Helper: resolve session ───────────────────────────────────────────────

// MustSession returns the named session or an error.
func MustSession(env *Env, name string) (*session.Session, error) {
	if name == "" {
		name = "main"
	}
	s, ok := env.Sessions.Get(name)
	if !ok {
		return nil, fmt.Errorf("session %q not found; call session_start first", name)
	}
	return s, nil
}

// JSONText marshals v and returns it as a text MCP result.
func JSONText(v interface{}) *mcp.CallToolResult {
	b, _ := json.MarshalIndent(v, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}
}

// _ suppresses unused import lint
var _ = fmt.Sprintf
