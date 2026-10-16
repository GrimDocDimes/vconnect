// toolset_debug.go — "debug" toolset
// ILA/VIO debug core insertion and hardware debug capture.
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterDebugToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_debug_core",
		Description: "Create an ILA or VIO debug core in the design.",
	}, createDebugCore(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_debug_core_property",
		Description: "Set properties on a debug core (e.g. C_DATA_DEPTH, C_NUM_OF_PROBES).",
	}, setDebugCoreProperty(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "connect_debug_port",
		Description: "Connect a net to a debug core probe port.",
	}, connectDebugPort(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "implement_debug_core",
		Description: "Run implement_debug_core to insert the debug hub and cores.",
	}, implementDebugCore(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_ila_run",
		Description: "Arm and run an ILA trigger capture on connected hardware.",
	}, hwILARun(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_ila_wait",
		Description: "Wait for ILA capture to complete and return status.",
	}, hwILAWait(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_vio_get",
		Description: "Read current VIO probe values from hardware.",
	}, hwVIOGet(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_vio_set",
		Description: "Drive VIO output probes to specified values.",
	}, hwVIOSet(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "write_debug_probes",
		Description: "Write the debug probes file (.ltx) after implementation.",
	}, writeDebugProbes(env))
}

type createDebugCoreIn struct {
	Name    string `json:"name"    jsonschema:"instance name for the core"`
	Type    string `json:"type"    jsonschema:"ila | vio"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type setDebugCorePropertyIn struct {
	Core       string            `json:"core"       jsonschema:"debug core instance name"`
	Properties map[string]string `json:"properties" jsonschema:"property name/value map"`
	Session    string            `json:"session"    jsonschema:"session name, default 'main'"`
}
type connectDebugPortIn struct {
	Core    string `json:"core"    jsonschema:"debug core name"`
	Port    string `json:"port"    jsonschema:"probe port, e.g. 'probe0'"`
	Net     string `json:"net"     jsonschema:"net to connect"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type implementDebugCoreIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type hwILARunIn struct {
	ILA     string `json:"ila"     jsonschema:"ILA instance name (optional, uses first)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type hwILAWaitIn struct {
	ILA     string `json:"ila"     jsonschema:"ILA instance name (optional, uses first)"`
	Timeout int    `json:"timeout" jsonschema:"timeout in seconds, default 30"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type hwVIOGetIn struct {
	VIO     string `json:"vio"     jsonschema:"VIO instance name (optional, uses first)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type hwVIOSetIn struct {
	VIO     string            `json:"vio"     jsonschema:"VIO instance name (optional, uses first)"`
	Values  map[string]string `json:"values"  jsonschema:"output probe name -> value map"`
	Session string            `json:"session" jsonschema:"session name, default 'main'"`
}
type writeDebugProbesIn struct {
	File    string `json:"file"    jsonschema:"output .ltx file path"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func createDebugCore(env *Env) mcp.ToolHandlerFor[createDebugCoreIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in createDebugCoreIn) (*mcp.CallToolResult, bool, error) {
		if in.Name == "" || in.Type == "" {
			return nil, false, fmt.Errorf("name and type are required")
		}
		coreType := map[string]string{
			"ila": "ila",
			"vio": "vio",
		}[in.Type]
		if coreType == "" {
			return nil, false, fmt.Errorf("type must be 'ila' or 'vio'")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("create_debug_core {%s} %s", in.Name, coreType)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("create_debug_core: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setDebugCoreProperty(env *Env) mcp.ToolHandlerFor[setDebugCorePropertyIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setDebugCorePropertyIn) (*mcp.CallToolResult, bool, error) {
		if in.Core == "" || len(in.Properties) == 0 {
			return nil, false, fmt.Errorf("core and properties are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		for k, v := range in.Properties {
			script := fmt.Sprintf("set_property %s {%s} [get_debug_cores {%s}]", k, v, in.Core)
			res, err := s.Exec(ctx, script, 10*time.Second)
			if err != nil {
				return nil, false, err
			}
			if !res.OK {
				return nil, false, fmt.Errorf("set_debug_core_property %s: %s", k, res.Error)
			}
		}
		return nil, true, nil
	}
}

func connectDebugPort(env *Env) mcp.ToolHandlerFor[connectDebugPortIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in connectDebugPortIn) (*mcp.CallToolResult, bool, error) {
		if in.Core == "" || in.Port == "" || in.Net == "" {
			return nil, false, fmt.Errorf("core, port, and net are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("connect_debug_port %s/%s [get_nets {%s}]", in.Core, in.Port, in.Net)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("connect_debug_port: %s", res.Error)
		}
		return nil, true, nil
	}
}

func implementDebugCore(env *Env) mcp.ToolHandlerFor[implementDebugCoreIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in implementDebugCoreIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		res, err := s.Exec(ctx, "implement_debug_core", 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("implement_debug_core: %s", res.Error)
		}
		return nil, true, nil
	}
}

func hwILARun(env *Env) mcp.ToolHandlerFor[hwILARunIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwILARunIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		ila := ilaExpr(in.ILA)
		script := fmt.Sprintf("run_hw_ila %s", ila)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("hw_ila_run: %s", res.Error)
		}
		return nil, true, nil
	}
}

func hwILAWait(env *Env) mcp.ToolHandlerFor[hwILAWaitIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwILAWaitIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		t := in.Timeout
		if t == 0 {
			t = 30
		}
		ila := ilaExpr(in.ILA)
		script := fmt.Sprintf("wait_on_hw_ila -timeout %d %s", t, ila)
		res, err := s.Exec(ctx, script, time.Duration(t+5)*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("hw_ila_wait: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func hwVIOGet(env *Env) mcp.ToolHandlerFor[hwVIOGetIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwVIOGetIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		vio := vioExpr(in.VIO)
		script := fmt.Sprintf("refresh_hw_vio %s\nget_property INPUT_VALUE [get_hw_probes -of_objects %s]", vio, vio)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("hw_vio_get: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func hwVIOSet(env *Env) mcp.ToolHandlerFor[hwVIOSetIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwVIOSetIn) (*mcp.CallToolResult, bool, error) {
		if len(in.Values) == 0 {
			return nil, false, fmt.Errorf("values is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		vio := vioExpr(in.VIO)
		var script string
		for probe, val := range in.Values {
			script += fmt.Sprintf("set_property OUTPUT_VALUE {%s} [get_hw_probes {%s} -of_objects %s]\n",
				val, probe, vio)
		}
		script += fmt.Sprintf("commit_hw_vio [get_hw_probes -of_objects %s]", vio)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("hw_vio_set: %s", res.Error)
		}
		return nil, true, nil
	}
}

func writeDebugProbes(env *Env) mcp.ToolHandlerFor[writeDebugProbesIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in writeDebugProbesIn) (*mcp.CallToolResult, bool, error) {
		if in.File == "" {
			return nil, false, fmt.Errorf("file is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("write_debug_probes -force {%s}", in.File)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("write_debug_probes: %s", res.Error)
		}
		return nil, true, nil
	}
}

func ilaExpr(name string) string {
	if name != "" {
		return fmt.Sprintf("[get_hw_ilas {%s}]", name)
	}
	return "[lindex [get_hw_ilas] 0]"
}

func vioExpr(name string) string {
	if name != "" {
		return fmt.Sprintf("[get_hw_vios {%s}]", name)
	}
	return "[lindex [get_hw_vios] 0]"
}
