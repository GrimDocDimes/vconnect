// toolset_constraints.go — "constraints" toolset
// Owns XDC files directly. Atomic writes: write temp, fsync, rename.
// Preserves comments and ordering on edit.
package tools

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterConstraintsToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "read_xdc",
		Description: "Return the contents of an XDC constraint file.",
	}, readXDC(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "append_xdc",
		Description: "Append constraint lines to an XDC file. Uses atomic write (write temp → fsync → rename). Preserves existing content.",
	}, appendXDC(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "replace_xdc_constraint",
		Description: "Replace a specific constraint line in an XDC file by matching a prefix pattern. Atomic write.",
	}, replaceXDCConstraint(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "reload_xdc",
		Description: "Reload constraint files in Vivado after editing (calls read_xdc in Tcl, not the MCP tool).",
	}, reloadXDC(env))
}

type readXDCIn struct {
	Path string `json:"path" jsonschema:"path to the XDC file"`
}

type appendXDCIn struct {
	Path  string   `json:"path"  jsonschema:"path to the XDC file"`
	Lines []string `json:"lines" jsonschema:"constraint lines to append"`
}

type replaceXDCConstraintIn struct {
	Path       string `json:"path"       jsonschema:"path to the XDC file"`
	MatchPfx   string `json:"match_pfx"  jsonschema:"prefix to match the target line (e.g. 'set_property -dict { PACKAGE_PIN E3 ')"`
	NewLine    string `json:"new_line"   jsonschema:"replacement line (full constraint)"`
}

type reloadXDCIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func readXDC(env *Env) mcp.ToolHandlerFor[readXDCIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in readXDCIn) (*mcp.CallToolResult, string, error) {
		if in.Path == "" {
			return nil, "", fmt.Errorf("path is required")
		}
		data, err := os.ReadFile(in.Path)
		if err != nil {
			return nil, "", fmt.Errorf("read_xdc: %w", err)
		}
		return nil, string(data), nil
	}
}

func appendXDC(env *Env) mcp.ToolHandlerFor[appendXDCIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in appendXDCIn) (*mcp.CallToolResult, bool, error) {
		if in.Path == "" {
			return nil, false, fmt.Errorf("path is required")
		}
		if len(in.Lines) == 0 {
			return nil, false, fmt.Errorf("lines is required")
		}
		// Read existing content
		existing, _ := os.ReadFile(in.Path)
		content := string(existing)
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += strings.Join(in.Lines, "\n") + "\n"
		if err := atomicWriteFile(in.Path, []byte(content)); err != nil {
			return nil, false, fmt.Errorf("append_xdc: %w", err)
		}
		return nil, true, nil
	}
}

func replaceXDCConstraint(env *Env) mcp.ToolHandlerFor[replaceXDCConstraintIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in replaceXDCConstraintIn) (*mcp.CallToolResult, bool, error) {
		if in.Path == "" || in.MatchPfx == "" {
			return nil, false, fmt.Errorf("path and match_pfx are required")
		}
		data, err := os.ReadFile(in.Path)
		if err != nil {
			return nil, false, fmt.Errorf("read file: %w", err)
		}

		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		var lines []string
		replaced := 0
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, in.MatchPfx) {
				lines = append(lines, in.NewLine)
				replaced++
			} else {
				lines = append(lines, line)
			}
		}
		if replaced == 0 {
			return nil, false, fmt.Errorf("no line matching prefix %q found in %s", in.MatchPfx, in.Path)
		}

		content := strings.Join(lines, "\n") + "\n"
		if err := atomicWriteFile(in.Path, []byte(content)); err != nil {
			return nil, false, fmt.Errorf("replace_xdc_constraint: %w", err)
		}
		return nil, true, nil
	}
}

func reloadXDC(env *Env) mcp.ToolHandlerFor[reloadXDCIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in reloadXDCIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		// Reload all XDC files in the constraints fileset
		script := `
set constrs [get_filesets constrs_1]
foreach f [get_files -of_objects $constrs] {
    if {[file extension $f] eq ".xdc"} {
        read_xdc -force $f
    }
}
`
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("reload_xdc: %s", res.Error)
		}
		return nil, true, nil
	}
}

// atomicWriteFile writes data to path atomically: write temp → sync → rename.
func atomicWriteFile(path string, data []byte) error {
	tmp := path + ".vconn.tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}
