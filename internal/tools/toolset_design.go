// toolset_design.go — "design" toolset
// Design object queries and property access: get_cells, get_nets, get_ports,
// get/set property, report_io, report_exceptions, write_verilog, write_vhdl.
package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterDesignToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_cells",
		Description: "Query cells in the design by name glob or hierarchy path.",
	}, getCells(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_nets",
		Description: "Query nets in the design by name glob.",
	}, getNets(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_ports",
		Description: "Query ports (top-level I/O) by name glob.",
	}, getPorts(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_property",
		Description: "Get a property value from a design object.",
	}, getPropertyObj(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_property",
		Description: "Set one or more properties on a design object.",
	}, setPropertyObj(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "report_io",
		Description: "Run report_io and return package pin / I/O standard assignments.",
	}, reportIO(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "report_exceptions",
		Description: "Run report_exceptions to list all timing exceptions (false paths, multicycles).",
	}, reportExceptions(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "write_verilog",
		Description: "Export the post-synthesis/implementation netlist as Verilog.",
	}, writeVerilog(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "write_vhdl",
		Description: "Export the netlist as VHDL.",
	}, writeVHDL(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_runs",
		Description: "List all synthesis/implementation runs and their status.",
	}, getRuns(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_pblock",
		Description: "Create a floorplan pblock and assign cells to it.",
	}, createPblock(env))
}

type getCellsIn struct {
	Filter  string `json:"filter"      jsonschema:"glob filter for cell name, e.g. 'u_fifo/*'"`
	Hier    bool   `json:"hierarchical" jsonschema:"include hierarchical cells"`
	Session string `json:"session"     jsonschema:"session name, default 'main'"`
}
type getNetsIn struct {
	Filter  string `json:"filter"  jsonschema:"glob filter for net name"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type getPortsIn struct {
	Filter  string `json:"filter"  jsonschema:"glob filter for port name"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type getPropertyObjIn struct {
	Object  string `json:"object"   jsonschema:"Tcl expression returning object(s), e.g. '[get_ports clk]'"`
	Property string `json:"property" jsonschema:"property name, e.g. 'PACKAGE_PIN'"`
	Session string `json:"session"  jsonschema:"session name, default 'main'"`
}
type setPropertyObjIn struct {
	Object     string            `json:"object"     jsonschema:"Tcl expression returning object(s)"`
	Properties map[string]string `json:"properties" jsonschema:"property name/value pairs"`
	Session    string            `json:"session"    jsonschema:"session name, default 'main'"`
}
type reportIOIn struct {
	OutFile string `json:"out_file" jsonschema:"optional file path to save report"`
	Session string `json:"session"  jsonschema:"session name, default 'main'"`
}
type reportExceptionsIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type writeNetlistIn struct {
	File    string `json:"file"    jsonschema:"output file path"`
	Mode    string `json:"mode"    jsonschema:"write mode: hier|flat (default hier)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type getRunsIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type createPblockIn struct {
	Name    string   `json:"name"    jsonschema:"pblock name"`
	Cells   []string `json:"cells"   jsonschema:"cell names to add to pblock"`
	Range   string   `json:"range"   jsonschema:"device range, e.g. 'SLICE_X0Y0:SLICE_X9Y9'"`
	Session string   `json:"session" jsonschema:"session name, default 'main'"`
}

func getCells(env *Env) mcp.ToolHandlerFor[getCellsIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getCellsIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		filter := in.Filter
		if filter == "" {
			filter = "*"
		}
		hier := ""
		if in.Hier {
			hier = " -hierarchical"
		}
		script := fmt.Sprintf("get_cells%s -filter {NAME =~ {%s}}", hier, filter)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("get_cells: %s", res.Error)
		}
		return nil, splitTclList(res.Value), nil
	}
}

func getNets(env *Env) mcp.ToolHandlerFor[getNetsIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getNetsIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		filter := in.Filter
		if filter == "" {
			filter = "*"
		}
		script := fmt.Sprintf("get_nets -filter {NAME =~ {%s}}", filter)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("get_nets: %s", res.Error)
		}
		return nil, splitTclList(res.Value), nil
	}
}

func getPorts(env *Env) mcp.ToolHandlerFor[getPortsIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getPortsIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		filter := in.Filter
		if filter == "" {
			filter = "*"
		}
		script := fmt.Sprintf("get_ports {%s}", filter)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("get_ports: %s", res.Error)
		}
		return nil, splitTclList(res.Value), nil
	}
}

func getPropertyObj(env *Env) mcp.ToolHandlerFor[getPropertyObjIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getPropertyObjIn) (*mcp.CallToolResult, string, error) {
		if in.Object == "" || in.Property == "" {
			return nil, "", fmt.Errorf("object and property are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		script := fmt.Sprintf("get_property %s %s", in.Property, in.Object)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("get_property: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func setPropertyObj(env *Env) mcp.ToolHandlerFor[setPropertyObjIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setPropertyObjIn) (*mcp.CallToolResult, bool, error) {
		if in.Object == "" || len(in.Properties) == 0 {
			return nil, false, fmt.Errorf("object and properties are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var props []string
		for k, v := range in.Properties {
			props = append(props, fmt.Sprintf("%s {%s}", k, v))
		}
		script := fmt.Sprintf("set_property -dict [list %s] %s", strings.Join(props, " "), in.Object)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_property: %s", res.Error)
		}
		return nil, true, nil
	}
}

func reportIO(env *Env) mcp.ToolHandlerFor[reportIOIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in reportIOIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		var script string
		if in.OutFile != "" {
			script = fmt.Sprintf("report_io -quiet -file {%s}", in.OutFile)
		} else {
			script = "report_io -quiet"
		}
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("report_io: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func reportExceptions(env *Env) mcp.ToolHandlerFor[reportExceptionsIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in reportExceptionsIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		res, err := s.Exec(ctx, "report_exceptions -quiet", 30*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("report_exceptions: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func writeVerilog(env *Env) mcp.ToolHandlerFor[writeNetlistIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in writeNetlistIn) (*mcp.CallToolResult, bool, error) {
		if in.File == "" {
			return nil, false, fmt.Errorf("file is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		mode := in.Mode
		if mode == "" || mode == "hier" {
			mode = "-hier"
		} else {
			mode = ""
		}
		script := fmt.Sprintf("write_verilog -force %s {%s}", mode, in.File)
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("write_verilog: %s", res.Error)
		}
		return nil, true, nil
	}
}

func writeVHDL(env *Env) mcp.ToolHandlerFor[writeNetlistIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in writeNetlistIn) (*mcp.CallToolResult, bool, error) {
		if in.File == "" {
			return nil, false, fmt.Errorf("file is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("write_vhdl -force {%s}", in.File)
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("write_vhdl: %s", res.Error)
		}
		return nil, true, nil
	}
}

func getRuns(env *Env) mcp.ToolHandlerFor[getRunsIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in getRunsIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		script := `
set out {}
foreach r [get_runs] {
    set status [get_property STATUS $r]
    set step   [get_property CURRENT_STEP $r]
    lappend out "$r  status=$status  step=$step"
}
join $out \n`
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("get_runs: %s", res.Error)
		}
		var lines []string
		for _, l := range strings.Split(res.Value, "\n") {
			if strings.TrimSpace(l) != "" {
				lines = append(lines, l)
			}
		}
		return nil, lines, nil
	}
}

func createPblock(env *Env) mcp.ToolHandlerFor[createPblockIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in createPblockIn) (*mcp.CallToolResult, bool, error) {
		if in.Name == "" {
			return nil, false, fmt.Errorf("name is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("create_pblock {%s}\n", in.Name)
		if in.Range != "" {
			script += fmt.Sprintf("resize_pblock [get_pblocks {%s}] -add {%s}\n", in.Name, in.Range)
		}
		for _, c := range in.Cells {
			script += fmt.Sprintf("add_cells_to_pblock [get_pblocks {%s}] [get_cells {%s}]\n", in.Name, c)
		}
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("create_pblock: %s", res.Error)
		}
		return nil, true, nil
	}
}
