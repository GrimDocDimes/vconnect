// toolset_sim.go — "sim" toolset: simulation lifecycle
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterSimToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "launch_sim",
		Description: "Launch simulation for the active simulation run (launch_simulation).",
	}, launchSim(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "run_sim",
		Description: "Run simulation for a given time or run -all.",
	}, runSim(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "restart_sim",
		Description: "Restart simulation from time zero.",
	}, restartSim(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "close_sim",
		Description: "Close the current simulation.",
	}, closeSim(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_wave",
		Description: "Add signals to the waveform view (add_wave).",
	}, addWave(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "log_wave",
		Description: "Enable wave logging to a WDB file.",
	}, logWave(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_sim_target",
		Description: "Set simulation fileset, top, and mode (behavioral/post-synth/post-impl).",
	}, setSimTarget(env))
}

type launchSimIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type runSimIn struct {
	Time    string `json:"time"    jsonschema:"time to run, e.g. '1000ns', or 'all' to run until finish"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type restartSimIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type closeSimIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type addWaveIn struct {
	Paths   []string `json:"paths"   jsonschema:"HDL signal paths to add, e.g. ['/tb/clk', '/tb/rst']"`
	Session string   `json:"session" jsonschema:"session name, default 'main'"`
}
type logWaveIn struct {
	File    string `json:"file"    jsonschema:"output WDB file path"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type setSimTargetIn struct {
	Top     string `json:"top"     jsonschema:"simulation top module"`
	Mode    string `json:"mode"    jsonschema:"behavioral | post-synthesis | post-implementation"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func launchSim(env *Env) mcp.ToolHandlerFor[launchSimIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in launchSimIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		res, err := s.Exec(ctx, "launch_simulation", 120*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("launch_sim: %s", res.Error)
		}
		return nil, true, nil
	}
}

func runSim(env *Env) mcp.ToolHandlerFor[runSimIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in runSimIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		t := in.Time
		if t == "" || t == "all" {
			t = "-all"
		}
		script := fmt.Sprintf("run %s", t)
		res, err := s.Exec(ctx, script, 2*time.Hour)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("run_sim: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func restartSim(env *Env) mcp.ToolHandlerFor[restartSimIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in restartSimIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		res, err := s.Exec(ctx, "restart", 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("restart_sim: %s", res.Error)
		}
		return nil, true, nil
	}
}

func closeSim(env *Env) mcp.ToolHandlerFor[closeSimIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in closeSimIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		res, err := s.Exec(ctx, "close_sim", 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("close_sim: %s", res.Error)
		}
		return nil, true, nil
	}
}

func addWave(env *Env) mcp.ToolHandlerFor[addWaveIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in addWaveIn) (*mcp.CallToolResult, bool, error) {
		if len(in.Paths) == 0 {
			return nil, false, fmt.Errorf("paths is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		for _, p := range in.Paths {
			script := fmt.Sprintf("add_wave %s", p)
			res, err := s.Exec(ctx, script, 10*time.Second)
			if err != nil {
				return nil, false, err
			}
			if !res.OK {
				return nil, false, fmt.Errorf("add_wave %s: %s", p, res.Error)
			}
		}
		return nil, true, nil
	}
}

func logWave(env *Env) mcp.ToolHandlerFor[logWaveIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in logWaveIn) (*mcp.CallToolResult, bool, error) {
		if in.File == "" {
			return nil, false, fmt.Errorf("file is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("log_wave -r /* -to_file {%s}", in.File)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("log_wave: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setSimTarget(env *Env) mcp.ToolHandlerFor[setSimTargetIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setSimTargetIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var script string
		if in.Top != "" {
			script += fmt.Sprintf("set_property top {%s} [get_filesets sim_1]\n", in.Top)
		}
		if in.Mode != "" {
			modeMap := map[string]string{
				"behavioral":       "behavioral",
				"post-synthesis":   "post-synthesis",
				"post-synth":       "post-synthesis",
				"post-implementation": "post-implementation",
				"post-impl":        "post-implementation",
			}
			m, ok := modeMap[in.Mode]
			if !ok {
				return nil, false, fmt.Errorf("invalid mode %q; use behavioral, post-synthesis, or post-implementation", in.Mode)
			}
			script += fmt.Sprintf("set_property -name {xsim.simulate.runtime} -value {} -objects [get_filesets sim_1]\n")
			script += fmt.Sprintf("set_property -name {xsim.compile.xvlog.more_options} -value {} -objects [get_filesets sim_1]\n")
			_ = m // mode set via launch_simulation -mode flag
			script += fmt.Sprintf("# mode %s applies at launch_simulation -mode %s\n", in.Mode, m)
		}
		if script == "" {
			return nil, false, fmt.Errorf("top or mode is required")
		}
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_sim_target: %s", res.Error)
		}
		return nil, true, nil
	}
}
