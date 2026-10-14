// toolset_reports.go — "reports" toolset
package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func RegisterReportsToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "timing_summary",
		Description: "Run report_timing_summary and return the report text.",
	}, timingSummary(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "utilization",
		Description: "Run report_utilization and return the report text.",
	}, utilization(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "power_report",
		Description: "Run report_power and return the report text.",
	}, powerReport(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "drc_report",
		Description: "Run report_drc and return violations.",
	}, drcReport(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "methodology_report",
		Description: "Run report_methodology and return results.",
	}, methodologyReport(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "cdc_report",
		Description: "Run report_cdc (clock domain crossing) and return results.",
	}, cdcReport(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "clock_interaction",
		Description: "Run report_clock_interaction and return results.",
	}, clockInteraction(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "critical_paths",
		Description: "Run report_timing -nworst N to get the N worst timing paths.",
	}, criticalPaths(env))
}

type reportIn struct {
	Session  string `json:"session"   jsonschema:"session name, default 'main'"`
	OutFile  string `json:"out_file"  jsonschema:"optional file path to save the report"`
}

type criticalPathsIn struct {
	N       int    `json:"n"       jsonschema:"number of worst paths to return, default 10"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func runReport(env *Env, sessName, script string, timeout time.Duration) (string, error) {
	ctx := context.Background()
	s, err := MustSession(env, sessName)
	if err != nil {
		return "", err
	}
	res, err := s.Exec(ctx, script, timeout)
	if err != nil {
		return "", err
	}
	if !res.OK {
		return "", fmt.Errorf("report error: %s", res.Error)
	}
	return res.Value, nil
}

func reportHandler(env *Env, tclFmt string, timeout time.Duration) mcp.ToolHandlerFor[reportIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in reportIn) (*mcp.CallToolResult, string, error) {
		var script string
		if in.OutFile != "" {
			script = fmt.Sprintf(tclFmt, fmt.Sprintf("-file {%s}", in.OutFile))
		} else {
			// Use a temp file and read it back
			tmp := filepath.Join(env.Config.TmpDir, "rpt_tmp.txt")
			script = fmt.Sprintf(tclFmt, fmt.Sprintf("-file {%s}", tmp))
			s, err := MustSession(env, in.Session)
			if err != nil {
				return nil, "", err
			}
			res, err := s.Exec(ctx, script, timeout)
			if err != nil {
				return nil, "", err
			}
			if !res.OK {
				return nil, "", fmt.Errorf("report: %s", res.Error)
			}
			// Read file contents
			readScript := fmt.Sprintf(`set fh [open {%s} r]; set c [read $fh]; close $fh; return $c`, tmp)
			rres, rerr := s.Exec(ctx, readScript, 10*time.Second)
			if rerr != nil {
				return nil, res.Value, nil // fallback: return Tcl return value
			}
			return nil, rres.Value, nil
		}
		text, err := runReport(env, in.Session, script, timeout)
		return nil, text, err
	}
}

func timingSummary(env *Env) mcp.ToolHandlerFor[reportIn, string] {
	return reportHandler(env, "report_timing_summary -quiet %s", 120*time.Second)
}

func utilization(env *Env) mcp.ToolHandlerFor[reportIn, string] {
	return reportHandler(env, "report_utilization -quiet %s", 60*time.Second)
}

func powerReport(env *Env) mcp.ToolHandlerFor[reportIn, string] {
	return reportHandler(env, "report_power -quiet %s", 120*time.Second)
}

func drcReport(env *Env) mcp.ToolHandlerFor[reportIn, string] {
	return reportHandler(env, "report_drc -quiet %s", 60*time.Second)
}

func methodologyReport(env *Env) mcp.ToolHandlerFor[reportIn, string] {
	return reportHandler(env, "report_methodology -quiet %s", 60*time.Second)
}

func cdcReport(env *Env) mcp.ToolHandlerFor[reportIn, string] {
	return reportHandler(env, "report_cdc -quiet %s", 60*time.Second)
}

func clockInteraction(env *Env) mcp.ToolHandlerFor[reportIn, string] {
	return reportHandler(env, "report_clock_interaction -quiet %s", 60*time.Second)
}

func criticalPaths(env *Env) mcp.ToolHandlerFor[criticalPathsIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in criticalPathsIn) (*mcp.CallToolResult, string, error) {
		n := in.N
		if n <= 0 {
			n = 10
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		tmp := filepath.Join(env.Config.TmpDir, "critical_paths.txt")
		script := fmt.Sprintf(`report_timing -nworst %d -quiet -file {%s}`, n, tmp)
		res, err := s.Exec(ctx, script, 120*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("report_timing: %s", res.Error)
		}
		readScript := fmt.Sprintf(`set fh [open {%s} r]; set c [read $fh]; close $fh; return $c`, tmp)
		rres, _ := s.Exec(ctx, readScript, 10*time.Second)
		if rres != nil && rres.OK {
			return nil, rres.Value, nil
		}
		return nil, res.Value, nil
	}
}
