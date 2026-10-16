// toolset_nonproject.go — "nonproject" toolset
// Non-project mode (script-based) flow: read sources, synth, opt, place, route, write_bitstream.
package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterNonProjectToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "read_verilog",
		Description: "Read Verilog source files into the in-memory design (non-project mode).",
	}, readVerilog(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "read_vhdl",
		Description: "Read VHDL source files into the in-memory design (non-project mode).",
	}, readVHDL(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "read_xdc_file",
		Description: "Read an XDC constraint file into the in-memory design.",
	}, readXDCFile(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "synth_design",
		Description: "Run synthesis in non-project mode (synth_design -top <top> -part <part>).",
	}, synthDesign(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "opt_design",
		Description: "Run logic optimisation (opt_design) after synthesis.",
	}, optDesign(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "place_design",
		Description: "Run placement (place_design).",
	}, placeDesign(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "phys_opt_design",
		Description: "Run physical optimisation (phys_opt_design) after placement.",
	}, physOptDesign(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "route_design",
		Description: "Run routing (route_design).",
	}, routeDesign(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "write_bitstream",
		Description: "Generate bitstream file (write_bitstream).",
	}, writeBitstream(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "link_design",
		Description: "Link a post-synthesis netlist to the device (link_design -part <part>).",
	}, linkDesign(env))
}

type readSourcesIn struct {
	Files   []string `json:"files"   jsonschema:"list of source file paths"`
	Session string   `json:"session" jsonschema:"session name, default 'main'"`
}
type synthDesignIn struct {
	Top     string `json:"top"     jsonschema:"top module name"`
	Part    string `json:"part"    jsonschema:"device part, e.g. xc7a35tcpg236-1"`
	Flatten string `json:"flatten" jsonschema:"flatten hierarchy: none|full|rebuilt (default rebuilt)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type optDesignIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type placeDesignIn struct {
	Directive string `json:"directive" jsonschema:"placement directive (optional), e.g. 'AltSpreadLogic_high'"`
	Session   string `json:"session"   jsonschema:"session name, default 'main'"`
}
type physOptDesignIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type routeDesignIn struct {
	Directive string `json:"directive" jsonschema:"routing directive (optional)"`
	Session   string `json:"session"   jsonschema:"session name, default 'main'"`
}
type writeBitstreamIn struct {
	File    string `json:"file"    jsonschema:"output .bit file path"`
	Force   bool   `json:"force"   jsonschema:"overwrite existing file"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type linkDesignIn struct {
	Part    string `json:"part"    jsonschema:"device part"`
	Top     string `json:"top"     jsonschema:"top module name (optional)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func readVerilog(env *Env) mcp.ToolHandlerFor[readSourcesIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in readSourcesIn) (*mcp.CallToolResult, bool, error) {
		if len(in.Files) == 0 {
			return nil, false, fmt.Errorf("files is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var quoted []string
		for _, f := range in.Files {
			quoted = append(quoted, fmt.Sprintf("{%s}", f))
		}
		script := fmt.Sprintf("read_verilog %s", strings.Join(quoted, " "))
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("read_verilog: %s", res.Error)
		}
		return nil, true, nil
	}
}

func readVHDL(env *Env) mcp.ToolHandlerFor[readSourcesIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in readSourcesIn) (*mcp.CallToolResult, bool, error) {
		if len(in.Files) == 0 {
			return nil, false, fmt.Errorf("files is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var quoted []string
		for _, f := range in.Files {
			quoted = append(quoted, fmt.Sprintf("{%s}", f))
		}
		script := fmt.Sprintf("read_vhdl %s", strings.Join(quoted, " "))
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("read_vhdl: %s", res.Error)
		}
		return nil, true, nil
	}
}

type readXDCFileIn struct {
	Path    string `json:"path"    jsonschema:"path to the XDC file"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func readXDCFile(env *Env) mcp.ToolHandlerFor[readXDCFileIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in readXDCFileIn) (*mcp.CallToolResult, bool, error) {
		if in.Path == "" {
			return nil, false, fmt.Errorf("path is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("read_xdc {%s}", in.Path)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("read_xdc_file: %s", res.Error)
		}
		return nil, true, nil
	}
}

func synthDesign(env *Env) mcp.ToolHandlerFor[synthDesignIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in synthDesignIn) (*mcp.CallToolResult, string, error) {
		if in.Top == "" || in.Part == "" {
			return nil, "", fmt.Errorf("top and part are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		flatten := in.Flatten
		if flatten == "" {
			flatten = "rebuilt"
		}
		script := fmt.Sprintf("synth_design -top {%s} -part {%s} -flatten_hierarchy %s",
			in.Top, in.Part, flatten)
		res, err := s.Exec(ctx, script, 3*time.Hour)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("synth_design: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func optDesign(env *Env) mcp.ToolHandlerFor[optDesignIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in optDesignIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		res, err := s.Exec(ctx, "opt_design", 60*time.Minute)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("opt_design: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func placeDesign(env *Env) mcp.ToolHandlerFor[placeDesignIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in placeDesignIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		script := "place_design"
		if in.Directive != "" {
			script += fmt.Sprintf(" -directive %s", in.Directive)
		}
		res, err := s.Exec(ctx, script, 2*time.Hour)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("place_design: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func physOptDesign(env *Env) mcp.ToolHandlerFor[physOptDesignIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in physOptDesignIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		res, err := s.Exec(ctx, "phys_opt_design", 60*time.Minute)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("phys_opt_design: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func routeDesign(env *Env) mcp.ToolHandlerFor[routeDesignIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in routeDesignIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		script := "route_design"
		if in.Directive != "" {
			script += fmt.Sprintf(" -directive %s", in.Directive)
		}
		res, err := s.Exec(ctx, script, 3*time.Hour)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("route_design: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func writeBitstream(env *Env) mcp.ToolHandlerFor[writeBitstreamIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in writeBitstreamIn) (*mcp.CallToolResult, bool, error) {
		if in.File == "" {
			return nil, false, fmt.Errorf("file is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		force := ""
		if in.Force {
			force = " -force"
		}
		script := fmt.Sprintf("write_bitstream%s {%s}", force, in.File)
		res, err := s.Exec(ctx, script, 60*time.Minute)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("write_bitstream: %s", res.Error)
		}
		return nil, true, nil
	}
}

func linkDesign(env *Env) mcp.ToolHandlerFor[linkDesignIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in linkDesignIn) (*mcp.CallToolResult, bool, error) {
		if in.Part == "" {
			return nil, false, fmt.Errorf("part is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("link_design -part {%s}", in.Part)
		if in.Top != "" {
			script += fmt.Sprintf(" -top {%s}", in.Top)
		}
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("link_design: %s", res.Error)
		}
		return nil, true, nil
	}
}
