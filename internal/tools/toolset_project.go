// toolset_project.go — "project" toolset
package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterProjectToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_project",
		Description: "Create a new Vivado project. Never hardcodes part — supply part or board explicitly.",
	}, createProject(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "open_project",
		Description: "Open an existing .xpr project file.",
	}, openProject(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "close_project",
		Description: "Close the currently open project.",
	}, closeProject(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_sources",
		Description: "Add HDL source files or directories to the project.",
	}, addSources(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_top",
		Description: "Set the top-level module/entity for synthesis.",
	}, setTop(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_compile_order",
		Description: "Call update_compile_order -fileset sources_1 to recompute file order.",
	}, updateCompileOrder(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_files",
		Description: "List all source, constraint, and simulation files in the project.",
	}, listFiles(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "export_project_tcl",
		Description: "Export a reproducible Tcl rebuild script via write_project_tcl.",
	}, exportProjectTcl(env))
}

// ── Input types ────────────────────────────────────────────────────────────

type createProjectIn struct {
	Name      string `json:"name"       jsonschema:"project name"`
	Dir       string `json:"dir"        jsonschema:"project directory path"`
	Part      string `json:"part"       jsonschema:"FPGA part (optional if board is set)"`
	Board     string `json:"board"      jsonschema:"board_part string (optional if part is set)"`
	InMemory  bool   `json:"in_memory"  jsonschema:"create in-memory (non-project) mode if true"`
	Session   string `json:"session"    jsonschema:"session name, default 'main'"`
}

type openProjectIn struct {
	Path    string `json:"path"    jsonschema:"path to .xpr file"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type closeProjectIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type addSourcesIn struct {
	Files   []string `json:"files"   jsonschema:"list of file paths or directories to add"`
	Fileset string   `json:"fileset" jsonschema:"fileset name, default 'sources_1' (use 'constrs_1' for XDC, 'sim_1' for sim)"`
	Session string   `json:"session" jsonschema:"session name, default 'main'"`
}

type setTopIn struct {
	Top     string `json:"top"     jsonschema:"top-level module name"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type updateCompileOrderIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type listFilesIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type exportProjectTclIn struct {
	OutputFile string `json:"output_file" jsonschema:"path for the output Tcl script"`
	Session    string `json:"session"     jsonschema:"session name, default 'main'"`
}

// ── Handlers ───────────────────────────────────────────────────────────────

func createProject(env *Env) mcp.ToolHandlerFor[createProjectIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in createProjectIn) (*mcp.CallToolResult, bool, error) {
		if in.Name == "" || in.Dir == "" {
			return nil, false, fmt.Errorf("name and dir are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var sb strings.Builder
		if in.InMemory {
			fmt.Fprintf(&sb, "create_project -in_memory -part {%s}", in.Part)
		} else {
			fmt.Fprintf(&sb, "create_project {%s} {%s}", in.Name, in.Dir)
			if in.Part != "" {
				fmt.Fprintf(&sb, " -part {%s}", in.Part)
			}
		}
		res, err := s.Exec(ctx, sb.String(), 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("create_project: %s", res.Error)
		}
		if in.Board != "" {
			bscript := fmt.Sprintf("set_property board_part {%s} [current_project]", in.Board)
			bres, berr := s.Exec(ctx, bscript, 15*time.Second)
			if berr != nil {
				return nil, false, berr
			}
			if !bres.OK {
				return nil, false, fmt.Errorf("set board_part: %s", bres.Error)
			}
		}
		return nil, true, nil
	}
}

func openProject(env *Env) mcp.ToolHandlerFor[openProjectIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in openProjectIn) (*mcp.CallToolResult, bool, error) {
		if in.Path == "" {
			return nil, false, fmt.Errorf("path is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("open_project {%s}", in.Path)
		res, err := s.Exec(ctx, script, 120*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("open_project: %s", res.Error)
		}
		return nil, true, nil
	}
}

func closeProject(env *Env) mcp.ToolHandlerFor[closeProjectIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in closeProjectIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		res, err := s.Exec(ctx, "close_project", 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("close_project: %s", res.Error)
		}
		return nil, true, nil
	}
}

func addSources(env *Env) mcp.ToolHandlerFor[addSourcesIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in addSourcesIn) (*mcp.CallToolResult, bool, error) {
		if len(in.Files) == 0 {
			return nil, false, fmt.Errorf("files list is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		fileset := in.Fileset
		if fileset == "" {
			fileset = "sources_1"
		}
		// build a Tcl list of paths
		var paths strings.Builder
		for _, f := range in.Files {
			paths.WriteString("{")
			paths.WriteString(f)
			paths.WriteString("} ")
		}
		script := fmt.Sprintf("add_files -fileset [get_filesets %s] [list %s]", fileset, paths.String())
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("add_files: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setTop(env *Env) mcp.ToolHandlerFor[setTopIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setTopIn) (*mcp.CallToolResult, bool, error) {
		if in.Top == "" {
			return nil, false, fmt.Errorf("top is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("set_property top {%s} [current_fileset]", in.Top)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_top: %s", res.Error)
		}
		return nil, true, nil
	}
}

func updateCompileOrder(env *Env) mcp.ToolHandlerFor[updateCompileOrderIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in updateCompileOrderIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		res, err := s.Exec(ctx, "update_compile_order -fileset sources_1", 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("update_compile_order: %s", res.Error)
		}
		return nil, true, nil
	}
}

type fileEntry struct {
	Path    string `json:"path"`
	Fileset string `json:"fileset"`
	Used    bool   `json:"used_in_synthesis"`
}

func listFiles(env *Env) mcp.ToolHandlerFor[listFilesIn, []fileEntry] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in listFilesIn) (*mcp.CallToolResult, []fileEntry, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		// Get files with their fileset
		script := `
set out {}
foreach fs [get_filesets] {
    foreach f [get_files -of_objects [get_filesets $fs]] {
        lappend out "$fs\t$f"
    }
}
join $out "\n"
`
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("list_files: %s", res.Error)
		}
		var entries []fileEntry
		for _, line := range strings.Split(res.Value, "\n") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 {
				entries = append(entries, fileEntry{
					Path:    strings.TrimSpace(parts[1]),
					Fileset: strings.TrimSpace(parts[0]),
				})
			}
		}
		return nil, entries, nil
	}
}

func exportProjectTcl(env *Env) mcp.ToolHandlerFor[exportProjectTclIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in exportProjectTclIn) (*mcp.CallToolResult, bool, error) {
		if in.OutputFile == "" {
			return nil, false, fmt.Errorf("output_file is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("write_project_tcl -force {%s}", in.OutputFile)
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("write_project_tcl: %s", res.Error)
		}
		return nil, true, nil
	}
}
