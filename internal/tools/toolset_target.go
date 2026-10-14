// toolset_target.go — "target" toolset: list_parts, list_boards, set_part, set_board
package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterTargetToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_parts",
		Description: "List available Xilinx/AMD device parts. Pass a filter glob (e.g. 'xc7a*') to narrow results. Queries Vivado via get_parts.",
	}, listParts(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_boards",
		Description: "List available board files visible to Vivado. Optionally filter by vendor/name substring.",
	}, listBoards(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_part",
		Description: "Set the target FPGA part for the current project (calls set_property part).",
	}, setPart(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_board",
		Description: "Set the target board for the current project (calls set_property board_part).",
	}, setBoard(env))
}

type listPartsIn struct {
	Filter  string `json:"filter"  jsonschema:"glob filter, e.g. 'xc7a*' (optional)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type listBoardsIn struct {
	Filter  string `json:"filter"  jsonschema:"substring filter on vendor+name (optional)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type setPartIn struct {
	Part    string `json:"part"    jsonschema:"exact part name, e.g. xc7a35tcpg236-1"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type setBoardIn struct {
	Board   string `json:"board"   jsonschema:"board_part string, e.g. digilentinc.com:nexys4_ddr:part0:1.1"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func listParts(env *Env) mcp.ToolHandlerFor[listPartsIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in listPartsIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		filter := in.Filter
		if filter == "" {
			filter = "*"
		}
		script := fmt.Sprintf("get_parts -filter {NAME =~ {%s}}", filter)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("vivado: %s", res.Error)
		}
		parts := splitTclList(res.Value)
		return nil, parts, nil
	}
}

func listBoards(env *Env) mcp.ToolHandlerFor[listBoardsIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in listBoardsIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		res, err := s.Exec(ctx, "get_boards", 30*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("vivado: %s", res.Error)
		}
		boards := splitTclList(res.Value)
		if in.Filter != "" {
			var filtered []string
			for _, b := range boards {
				if containsFold(b, in.Filter) {
					filtered = append(filtered, b)
				}
			}
			boards = filtered
		}
		return nil, boards, nil
	}
}

func setPart(env *Env) mcp.ToolHandlerFor[setPartIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setPartIn) (*mcp.CallToolResult, bool, error) {
		if in.Part == "" {
			return nil, false, fmt.Errorf("part is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("set_property part {%s} [current_project]", in.Part)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_part failed: %s", res.Error)
		}
		return nil, true, nil
	}
}

func setBoard(env *Env) mcp.ToolHandlerFor[setBoardIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setBoardIn) (*mcp.CallToolResult, bool, error) {
		if in.Board == "" {
			return nil, false, fmt.Errorf("board is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("set_property board_part {%s} [current_project]", in.Board)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_board failed: %s", res.Error)
		}
		return nil, true, nil
	}
}

// splitTclList splits a Tcl list string into Go strings.
// Handles simple space-separated and brace-quoted items.
func splitTclList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	i := 0
	for i < len(s) {
		// skip spaces
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == '{' {
			// brace-quoted word
			depth := 0
			start := i + 1
			for i < len(s) {
				if s[i] == '{' {
					depth++
				} else if s[i] == '}' {
					depth--
					if depth == 0 {
						out = append(out, s[start:i])
						i++
						break
					}
				}
				i++
			}
		} else {
			start := i
			for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
				i++
			}
			out = append(out, s[start:i])
		}
	}
	return out
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) &&
		func() bool {
			sl, subl := []rune(s), []rune(sub)
			for i := 0; i <= len(sl)-len(subl); i++ {
				match := true
				for j, c := range subl {
					sc := sl[i+j]
					if sc >= 'A' && sc <= 'Z' {
						sc += 32
					}
					dc := c
					if dc >= 'A' && dc <= 'Z' {
						dc += 32
					}
					if sc != dc {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
			return false
		}()
}
