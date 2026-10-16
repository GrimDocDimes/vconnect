// Package tools — toolset registration table.
// Call RegisterAllToolsets once at startup to populate the registry.
package tools

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/jhush/vconnect/internal/router"
)

// RegisterAllToolsets populates the registry with all on-demand toolsets.
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
		Name:        "timing_constraints",
		Description: "create_clock, create_generated_clock, set_input_delay, set_output_delay, set_false_path, set_multicycle_path, set_max_delay, check_timing, get_clocks.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterTimingConstraintsToolset(s, env)
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

	reg.Register(&router.Toolset{
		Name:        "sim",
		Description: "launch_sim, run_sim, restart_sim, close_sim, add_wave, log_wave, set_sim_target — simulation lifecycle.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterSimToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "nonproject",
		Description: "read_verilog, read_vhdl, read_xdc_file, synth_design, opt_design, place_design, phys_opt_design, route_design, write_bitstream, link_design — script-mode flow without a project file.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterNonProjectToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "design",
		Description: "get_cells, get_nets, get_ports, get_property, set_property, report_io, report_exceptions, write_verilog, write_vhdl, get_runs, create_pblock — design object queries and netlist export.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterDesignToolset(s, env)
		},
	})

	reg.Register(&router.Toolset{
		Name:        "debug",
		Description: "create_debug_core, set_debug_core_property, connect_debug_port, implement_debug_core, hw_ila_run, hw_ila_wait, hw_vio_get, hw_vio_set, write_debug_probes — ILA/VIO insertion and hardware capture.",
		Adder: func(s *mcp.Server, _ *router.Deps) {
			RegisterDebugToolset(s, env)
		},
	})
}
