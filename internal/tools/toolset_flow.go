// toolset_flow.go — "flow" toolset: launch_synth, launch_impl, launch_bitstream,
// set_strategy, run_status, reset_run, cancel_run
package tools

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/jhush/vconnect/internal/jobmgr"
)

func RegisterFlowToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "launch_synth",
		Description: "Launch synthesis. Returns a job_id for async polling. Uses launch_runs synth_1 + wait_on_run.",
	}, launchSynth(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "launch_impl",
		Description: "Launch implementation. Returns a job_id.",
	}, launchImpl(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "launch_bitstream",
		Description: "Launch bitstream generation. Returns a job_id.",
	}, launchBitstream(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "set_strategy",
		Description: "Set synth/impl strategy on a run (e.g. 'Vivado Synthesis Defaults' or 'Performance_ExplorePostRoutePhysOpt').",
	}, setStrategy(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "run_status",
		Description: "Return the status of a job_id returned by launch_*.",
	}, runStatus(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "reset_run",
		Description: "Reset a synthesis or implementation run to clear stale state.",
	}, resetRun(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "cancel_run",
		Description: "Cancel a running job by job_id.",
	}, cancelRun(env))
}

// ── Input types ────────────────────────────────────────────────────────────

type launchSynthIn struct {
	Jobs    int    `json:"jobs"    jsonschema:"parallel jobs for synthesis, default 4"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type launchImplIn struct {
	Jobs    int    `json:"jobs"    jsonschema:"parallel jobs, default 4"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type launchBitstreamIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type setStrategyIn struct {
	Run      string `json:"run"      jsonschema:"run name: synth_1 or impl_1"`
	Strategy string `json:"strategy" jsonschema:"Vivado strategy name"`
	Session  string `json:"session"  jsonschema:"session name, default 'main'"`
}
type runStatusIn struct {
	JobID string `json:"job_id" jsonschema:"job_id returned by launch_*"`
}
type resetRunIn struct {
	Run     string `json:"run"     jsonschema:"run name: synth_1 or impl_1"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type cancelRunIn struct {
	JobID string `json:"job_id" jsonschema:"job_id to cancel"`
}

// ── Handlers ───────────────────────────────────────────────────────────────

func launchSynth(env *Env) mcp.ToolHandlerFor[launchSynthIn, jobmgr.StatusFile] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in launchSynthIn) (*mcp.CallToolResult, jobmgr.StatusFile, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, jobmgr.StatusFile{}, err
		}
		jobs := in.Jobs
		if jobs <= 0 {
			jobs = 4
		}

		// Capture provenance before launching
		prov := captureProvenance(ctx, s)

		job, err := env.Jobs.Submit("synth", in.Session, prov, func(jobCtx context.Context, logPath, reportsDir string) error {
			lf, lerr := os.Create(logPath)
			if lerr != nil {
				return fmt.Errorf("create log: %w", lerr)
			}
			defer lf.Close()

			script := fmt.Sprintf(`
launch_runs synth_1 -jobs %d
wait_on_run synth_1
set rpt [get_runs synth_1]
set status [get_property STATUS $rpt]
if {[string first "ERROR" $status] >= 0} {
    error "Synthesis failed: $status"
}
return $status
`, jobs)
			res, execErr := s.Exec(jobCtx, script, 3*time.Hour)
			if execErr != nil {
				return execErr
			}
			// Write messages to log
			for _, m := range res.Messages {
				fmt.Fprintf(lf, "[%s] [%s] %s\n", m.Severity, m.ID, m.Text)
			}
			if !res.OK {
				return fmt.Errorf("synth: %s", res.Error)
			}
			return nil
		})
		if err != nil {
			return nil, jobmgr.StatusFile{}, err
		}
		return nil, job.Snapshot(), nil
	}
}

func launchImpl(env *Env) mcp.ToolHandlerFor[launchImplIn, jobmgr.StatusFile] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in launchImplIn) (*mcp.CallToolResult, jobmgr.StatusFile, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, jobmgr.StatusFile{}, err
		}
		jobs := in.Jobs
		if jobs <= 0 {
			jobs = 4
		}
		prov := captureProvenance(ctx, s)
		job, err := env.Jobs.Submit("impl", in.Session, prov, func(jobCtx context.Context, logPath, _ string) error {
			script := fmt.Sprintf(`
launch_runs impl_1 -jobs %d
wait_on_run impl_1
set rpt [get_runs impl_1]
set status [get_property STATUS $rpt]
if {[string first "ERROR" $status] >= 0} {
    error "Implementation failed: $status"
}
return $status
`, jobs)
			res, execErr := s.Exec(jobCtx, script, 6*time.Hour)
			if execErr != nil {
				return execErr
			}
			if !res.OK {
				return fmt.Errorf("impl: %s", res.Error)
			}
			return nil
		})
		if err != nil {
			return nil, jobmgr.StatusFile{}, err
		}
		return nil, job.Snapshot(), nil
	}
}

func launchBitstream(env *Env) mcp.ToolHandlerFor[launchBitstreamIn, jobmgr.StatusFile] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in launchBitstreamIn) (*mcp.CallToolResult, jobmgr.StatusFile, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, jobmgr.StatusFile{}, err
		}
		prov := captureProvenance(ctx, s)
		job, err := env.Jobs.Submit("bitstream", in.Session, prov, func(jobCtx context.Context, logPath, _ string) error {
			script := `
launch_runs impl_1 -to_step write_bitstream
wait_on_run impl_1
set status [get_property STATUS [get_runs impl_1]]
if {[string first "ERROR" $status] >= 0} {
    error "Bitstream failed: $status"
}
return $status
`
			res, execErr := s.Exec(jobCtx, script, 6*time.Hour)
			if execErr != nil {
				return execErr
			}
			if !res.OK {
				return fmt.Errorf("bitstream: %s", res.Error)
			}
			return nil
		})
		if err != nil {
			return nil, jobmgr.StatusFile{}, err
		}
		return nil, job.Snapshot(), nil
	}
}

func setStrategy(env *Env) mcp.ToolHandlerFor[setStrategyIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in setStrategyIn) (*mcp.CallToolResult, bool, error) {
		if in.Run == "" || in.Strategy == "" {
			return nil, false, fmt.Errorf("run and strategy are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("set_property strategy {%s} [get_runs %s]", in.Strategy, in.Run)
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("set_strategy: %s", res.Error)
		}
		return nil, true, nil
	}
}

func runStatus(env *Env) mcp.ToolHandlerFor[runStatusIn, jobmgr.StatusFile] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in runStatusIn) (*mcp.CallToolResult, jobmgr.StatusFile, error) {
		if in.JobID == "" {
			return nil, jobmgr.StatusFile{}, fmt.Errorf("job_id is required")
		}
		job, err := env.Jobs.Get(in.JobID)
		if err != nil {
			return nil, jobmgr.StatusFile{}, err
		}
		return nil, job.Snapshot(), nil
	}
}

func resetRun(env *Env) mcp.ToolHandlerFor[resetRunIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in resetRunIn) (*mcp.CallToolResult, bool, error) {
		if in.Run == "" {
			return nil, false, fmt.Errorf("run is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("reset_run %s", in.Run)
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("reset_run: %s", res.Error)
		}
		return nil, true, nil
	}
}

func cancelRun(env *Env) mcp.ToolHandlerFor[cancelRunIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in cancelRunIn) (*mcp.CallToolResult, bool, error) {
		if in.JobID == "" {
			return nil, false, fmt.Errorf("job_id is required")
		}
		job, err := env.Jobs.Get(in.JobID)
		if err != nil {
			return nil, false, err
		}
		job.Cancel()
		return nil, true, nil
	}
}

// captureProvenance queries Vivado for current part and version for provenance.
func captureProvenance(ctx context.Context, s *session.Session) jobmgr.Provenance {
	p := jobmgr.Provenance{}
	if res, err := s.Exec(ctx, "vconn_version", 5*time.Second); err == nil && res.OK {
		p.VivadoVersion = res.Value
	}
	if res, err := s.Exec(ctx, "catch {get_property part [current_project]} r; set r", 5*time.Second); err == nil && res.OK {
		p.Part = strings.TrimSpace(res.Value)
	}
	if res, err := s.Exec(ctx, "catch {get_property board_part [current_project]} r; set r", 5*time.Second); err == nil && res.OK {
		p.Board = strings.TrimSpace(res.Value)
	}
	return p
}

