// Package tools — toolset registration table.
// Call RegisterAllToolsets once at startup to populate the registry.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/jhush/vconnect/internal/router"
)

// RegisterAllToolsets populates the registry with all on-demand toolsets.
// None of them are enabled at startup.
func RegisterAllToolsets(reg *router.Registry, srv *mcp.Server, env *Env) {
	reg.Register(&router.Toolset{
		Name:        "target",
		Description: "list_parts, list_boards, set_part, set_board — discover and configure the target device.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterTargetToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "project",
		Description: "create_project, open_project, close_project, add_sources, set_top, update_compile_order, list_files, export_project_tcl.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterProjectToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "flow",
		Description: "launch_synth, launch_impl, launch_bitstream, set_strategy, run_status, reset_run, cancel_run.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterFlowToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "reports",
		Description: "timing_summary, utilization, power_report, drc_report, methodology_report, cdc_report, clock_interaction, critical_paths.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterReportsToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "constraints",
		Description: "read_xdc, append_xdc, replace_xdc_constraint, reload_xdc — own XDC files with atomic writes.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterConstraintsToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "ip",
		Description: "list_ip_catalog, create_ip, configure_ip, generate_ip, upgrade_ip.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterIPToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "bd",
		Description: "create_bd, add_bd_cell, connect_bd_net, validate_bd, make_bd_wrapper — block design operations.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterBDToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "checkpoints",
		Description: "write_checkpoint, read_checkpoint — save and restore design state.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterCheckpointsToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "hardware",
		Description: "hw_open, hw_program, hw_list_devices, hw_close — hardware manager and device programming.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterHardwareToolset(s, env)
		},
	})
}
