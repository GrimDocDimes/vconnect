// toolset_timing_constraints.go — "timing_constraints" toolset
// Proper SDC/XDC timing constraint creation via Tcl.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTimingConstraintsToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_clock",
		Description: "Create a primary clock constraint on a port or pin.",
	}, createClock(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_generated_clock",
		Description: "Create a generated clock derived from a primary clock.",
	}, createGeneratedClock(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_input_delay",
		Description: "Set input delay constraint relative to a clock.",
	}, setInputDelay(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_output_delay",
		Description: "Set output delay constraint relative to a clock.",
	}, setOutputDelay(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_false_path",
		Description: "Mark a path as a false path (timing exception).",
	}, setFalsePath(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_multicycle_path",
		Description: "Set a multicycle path timing exception.",
	}, setMulticyclePath(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_max_delay",
		Description: "Set a maximum delay on a path.",
	}, setMaxDelay(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "check_timing",
		Description: "Run check_timing and return any unconstrained paths or issues.",
	}, checkTiming(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_clocks",
		Description: "List all clocks defined in the design.",
	}, getClocks(env))
}

type createClockIn struct {
	Name    string  `json:"name"    jsonschema:"clock name"`
	Period  float64 `json:"period"  jsonschema:"clock period in nanoseconds"`
	Port    string  `json:"port"    jsonschema:"target port or pin path"`
	Session string  `json:"session" jsonschema:"session name, default 'main'"`
}

type createGeneratedClockIn struct {
	Name        string  `json:"name"         jsonschema:"generated clock name"`
	Source      string  `json:"source"       jsonschema:"source clock pin path"`
	Target      string  `json:"target"       jsonschema:"target pin or port"`
	DivideBy    int     `json:"divide_by"    jsonschema:"divide factor (optional)"`
	MultiplyBy  int     `json:"multiply_by"  jsonschema:"multiply factor (optional)"`
	Session     string  `json:"session"      jsonschema:"session name, default 'main'"`
}

type setInputDelayIn struct {
	Delay   float64 `json:"delay"   jsonschema:"delay value in nanoseconds"`
	Clock   string  `json:"clock"   jsonschema:"reference clock name"`
	Ports   string  `json:"ports"   jsonschema:"port glob pattern, e.g. 'data_in*'"`
	Max     bool    `json:"max"     jsonschema:"apply as max delay (setup). default true"`
	Session string  `json:"session" jsonschema:"session name, default 'main'"`
}

type setOutputDelayIn struct {
	Delay   float64 `json:"delay"   jsonschema:"delay value in nanoseconds"`
	Clock   string  `json:"clock"   jsonschema:"reference clock name"`
	Ports   string  `json:"ports"   jsonschema:"port glob pattern"`
	Session string  `json:"session" jsonschema:"session name, default 'main'"`
}

type setFalsePathIn struct {
	From    string `json:"from"    jsonschema:"source cell/pin/clock (optional)"`
	To      string `json:"to"      jsonschema:"destination cell/pin (optional)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type setMulticyclePathIn struct {
	Cycles  int    `json:"cycles"  jsonschema:"number of cycles"`
	From    string `json:"from"    jsonschema:"source (optional)"`
	To      string `json:"to"      jsonschema:"destination (optional)"`
	Setup   bool   `json:"setup"   jsonschema:"apply to setup check"`
	Hold    bool   `json:"hold"    jsonschema:"apply to hold check"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type setMaxDelayIn struct {
	Delay   float64 `json:"delay"   jsonschema:"max delay in nanoseconds"`
	From    string  `json:"from"    jsonschema:"source"`
	To      string  `json:"to"      jsonschema:"destination"`
	DataPath bool   `json:"datapath_only" jsonschema:"add -datapath_only flag"`
	Session string  `json:"session" jsonschema:"session name, default 'main'"`
}

type checkTimingIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type getClocksIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func createClock(env *Env) mcp.ToolHandlerFor[createClockIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in createClockIn) (*mcp.CallToolResult, bool, error) {
		if in.Port == "" || in.Period == 0 {
			return nil, false, fmt.Errorf("port and period are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		name := in.Name
		if name == "" {
			name = "clk"
		}
		script := fmt.Sprintf("create_clock -name {%s} -period %.3f [get_ports {%s}]", name, in.Period, in.Port)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("create_clock: %s", res.Error)
		}
		return nil, true, nil
	}
}

func createGeneratedClock(env *Env) mcp.ToolHandlerFor[createGeneratedClockIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in createGeneratedClockIn) (*mcp.CallToolResult, bool, error) {
		if in.Source == "" || in.Target == "" {
			return nil, false, fmt.Errorf("source and target are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("create_generated_clock -name {%s} -source [get_pins {%s}]", in.Name, in.Source)
		if in.DivideBy > 0 {
			script += fmt.Sprintf(" -divide_by %d", in.DivideBy)
		}
		if in.MultiplyBy > 0 {
			script += fmt.Sprintf(" -multiply_by %d", in.MultiplyBy)
		}
		script += fmt.Sprintf(" [get_pins {%s}]", in.Target)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("create_generated_clock: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setInputDelay(env *Env) mcp.ToolHandlerFor[setInputDelayIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setInputDelayIn) (*mcp.CallToolResult, bool, error) {
		if in.Clock == "" || in.Ports == "" {
			return nil, false, fmt.Errorf("clock and ports are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		flag := "-max"
		if !in.Max {
			flag = "-min"
		}
		script := fmt.Sprintf("set_input_delay %s %.3f -clock [get_clocks {%s}] [get_ports {%s}]",
			flag, in.Delay, in.Clock, in.Ports)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_input_delay: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setOutputDelay(env *Env) mcp.ToolHandlerFor[setOutputDelayIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setOutputDelayIn) (*mcp.CallToolResult, bool, error) {
		if in.Clock == "" || in.Ports == "" {
			return nil, false, fmt.Errorf("clock and ports are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("set_output_delay -max %.3f -clock [get_clocks {%s}] [get_ports {%s}]",
			in.Delay, in.Clock, in.Ports)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_output_delay: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setFalsePath(env *Env) mcp.ToolHandlerFor[setFalsePathIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setFalsePathIn) (*mcp.CallToolResult, bool, error) {
		if in.From == "" && in.To == "" {
			return nil, false, fmt.Errorf("at least one of from or to is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := "set_false_path"
		if in.From != "" {
			script += fmt.Sprintf(" -from [get_cells {%s}]", in.From)
		}
		if in.To != "" {
			script += fmt.Sprintf(" -to [get_cells {%s}]", in.To)
		}
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_false_path: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setMulticyclePath(env *Env) mcp.ToolHandlerFor[setMulticyclePathIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setMulticyclePathIn) (*mcp.CallToolResult, bool, error) {
		if in.Cycles == 0 {
			return nil, false, fmt.Errorf("cycles is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("set_multicycle_path %d", in.Cycles)
		if in.Setup {
			script += " -setup"
		}
		if in.Hold {
			script += " -hold"
		}
		if in.From != "" {
			script += fmt.Sprintf(" -from [get_cells {%s}]", in.From)
		}
		if in.To != "" {
			script += fmt.Sprintf(" -to [get_cells {%s}]", in.To)
		}
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_multicycle_path: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setMaxDelay(env *Env) mcp.ToolHandlerFor[setMaxDelayIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setMaxDelayIn) (*mcp.CallToolResult, bool, error) {
		if in.Delay == 0 {
			return nil, false, fmt.Errorf("delay is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("set_max_delay %.3f", in.Delay)
		if in.DataPath {
			script += " -datapath_only"
		}
		if in.From != "" {
			script += fmt.Sprintf(" -from [get_cells {%s}]", in.From)
		}
		if in.To != "" {
			script += fmt.Sprintf(" -to [get_cells {%s}]", in.To)
		}
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_max_delay: %s", res.Error)
		}
		return nil, true, nil
	}
}

func checkTiming(env *Env) mcp.ToolHandlerFor[checkTimingIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in checkTimingIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		res, err := s.Exec(ctx, "check_timing -quiet", 30*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("check_timing: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func getClocks(env *Env) mcp.ToolHandlerFor[getClocksIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getClocksIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		res, err := s.Exec(ctx, "get_clocks", 15*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("get_clocks: %s", res.Error)
		}
		return nil, splitTclList(res.Value), nil
	}
}
