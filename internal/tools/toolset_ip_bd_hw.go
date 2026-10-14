// toolset_ip.go — "ip" toolset: list IP catalog, create IP, upgrade IP, export XCI
// toolset_bd.go — "bd" toolset: block design operations
// toolset_checkpoints.go — checkpoint save/restore
// toolset_hardware.go — hardware manager: connect, program, verify
package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── IP toolset ─────────────────────────────────────────────────────────────

func RegisterIPToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_ip_catalog",
		Description: "List available IP definitions from the Vivado IP catalog. Filter by name glob.",
	}, listIPCatalog(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_ip",
		Description: "Create an IP instance in the project (calls create_ip with VLNV).",
	}, createIP(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "configure_ip",
		Description: "Configure IP properties (calls set_property CONFIG.* on the IP).",
	}, configureIP(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "generate_ip",
		Description: "Generate IP output products.",
	}, generateIP(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "upgrade_ip",
		Description: "Upgrade one or all IP to the latest version.",
	}, upgradeIP(env))
}

type listIPCatalogIn struct {
	Filter  string `json:"filter"  jsonschema:"glob filter for IP name, e.g. 'clk_wiz*'"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type createIPIn struct {
	VLNV    string `json:"vlnv"    jsonschema:"IP VLNV, e.g. xilinx.com:ip:clk_wiz:6.0"`
	Name    string `json:"name"    jsonschema:"instance name"`
	Module  string `json:"module"  jsonschema:"module name (optional, defaults to name)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type configureIPIn struct {
	Name       string            `json:"name"       jsonschema:"IP instance name"`
	Properties map[string]string `json:"properties" jsonschema:"CONFIG.* properties to set"`
	Session    string            `json:"session"    jsonschema:"session name, default 'main'"`
}

type generateIPIn struct {
	Name    string `json:"name"    jsonschema:"IP instance name (or '*' for all)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

type upgradeIPIn struct {
	Name    string `json:"name"    jsonschema:"IP instance name (or '*' for all)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func listIPCatalog(env *Env) mcp.ToolHandlerFor[listIPCatalogIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in listIPCatalogIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		filter := in.Filter
		if filter == "" {
			filter = "*"
		}
		script := fmt.Sprintf("lsearch -all -inline [get_ipdefs] *%s*", filter)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("list_ip_catalog: %s", res.Error)
		}
		return nil, splitTclList(res.Value), nil
	}
}

func createIP(env *Env) mcp.ToolHandlerFor[createIPIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in createIPIn) (*mcp.CallToolResult, bool, error) {
		if in.VLNV == "" || in.Name == "" {
			return nil, false, fmt.Errorf("vlnv and name are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		module := in.Module
		if module == "" {
			module = in.Name
		}
		script := fmt.Sprintf("create_ip -name [lindex [split {%s} :] 2] -vendor [lindex [split {%s} :] 0] -library [lindex [split {%s} :] 1] -version [lindex [split {%s} :] 3] -module_name {%s}",
			in.VLNV, in.VLNV, in.VLNV, in.VLNV, module)
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("create_ip: %s", res.Error)
		}
		return nil, true, nil
	}
}

func configureIP(env *Env) mcp.ToolHandlerFor[configureIPIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in configureIPIn) (*mcp.CallToolResult, bool, error) {
		if in.Name == "" || len(in.Properties) == 0 {
			return nil, false, fmt.Errorf("name and properties are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var props []string
		for k, v := range in.Properties {
			props = append(props, fmt.Sprintf("%s {%s}", k, v))
		}
		script := fmt.Sprintf("set_property -dict [list %s] [get_ips {%s}]",
			strings.Join(props, " "), in.Name)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("configure_ip: %s", res.Error)
		}
		return nil, true, nil
	}
}

func generateIP(env *Env) mcp.ToolHandlerFor[generateIPIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in generateIPIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		target := in.Name
		if target == "" || target == "*" {
			target = "[get_ips]"
		} else {
			target = fmt.Sprintf("[get_ips {%s}]", target)
		}
		script := fmt.Sprintf("generate_target all %s", target)
		res, err := s.Exec(ctx, script, 300*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("generate_ip: %s", res.Error)
		}
		return nil, true, nil
	}
}

func upgradeIP(env *Env) mcp.ToolHandlerFor[upgradeIPIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in upgradeIPIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		target := in.Name
		if target == "" || target == "*" {
			target = "[get_ips]"
		} else {
			target = fmt.Sprintf("[get_ips {%s}]", target)
		}
		script := fmt.Sprintf("upgrade_ip %s", target)
		res, err := s.Exec(ctx, script, 120*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("upgrade_ip: %s", res.Error)
		}
		return nil, true, nil
	}
}

// ── Block Design toolset ───────────────────────────────────────────────────

func RegisterBDToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_bd",
		Description: "Create a new block design.",
	}, createBD(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_bd_cell",
		Description: "Add an IP cell to the block design.",
	}, addBDCell(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "connect_bd_net",
		Description: "Connect nets in the block design.",
	}, connectBDNet(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "validate_bd",
		Description: "Validate the block design (validate_bd_design).",
	}, validateBD(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "make_bd_wrapper",
		Description: "Generate and set as top the HDL wrapper for the block design.",
	}, makeBDWrapper(env))
}

type createBDIn struct {
	Name    string `json:"name"    jsonschema:"block design name"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type addBDCellIn struct {
	VLNV    string `json:"vlnv"    jsonschema:"IP VLNV"`
	Cell    string `json:"cell"    jsonschema:"cell instance name"`
	BD      string `json:"bd"      jsonschema:"block design name (optional, uses current)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type connectBDNetIn struct {
	Pins    []string `json:"pins"    jsonschema:"list of pin paths to connect"`
	Session string   `json:"session" jsonschema:"session name, default 'main'"`
}
type validateBDIn struct {
	BD      string `json:"bd"      jsonschema:"block design name (optional)"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type makeBDWrapperIn struct {
	BD      string `json:"bd"      jsonschema:"block design name"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func createBD(env *Env) mcp.ToolHandlerFor[createBDIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in createBDIn) (*mcp.CallToolResult, bool, error) {
		if in.Name == "" {
			return nil, false, fmt.Errorf("name is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("create_bd_design {%s}", in.Name)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("create_bd: %s", res.Error)
		}
		return nil, true, nil
	}
}

func addBDCell(env *Env) mcp.ToolHandlerFor[addBDCellIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in addBDCellIn) (*mcp.CallToolResult, bool, error) {
		if in.VLNV == "" || in.Cell == "" {
			return nil, false, fmt.Errorf("vlnv and cell are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("create_bd_cell -type ip -vlnv {%s} {%s}", in.VLNV, in.Cell)
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("add_bd_cell: %s", res.Error)
		}
		return nil, true, nil
	}
}

func connectBDNet(env *Env) mcp.ToolHandlerFor[connectBDNetIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in connectBDNetIn) (*mcp.CallToolResult, bool, error) {
		if len(in.Pins) < 2 {
			return nil, false, fmt.Errorf("at least two pins are required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var pinStrs []string
		for _, p := range in.Pins {
			pinStrs = append(pinStrs, fmt.Sprintf("[get_bd_pins {%s}]", p))
		}
		script := fmt.Sprintf("connect_bd_net %s", strings.Join(pinStrs, " "))
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("connect_bd_net: %s", res.Error)
		}
		return nil, true, nil
	}
}

func validateBD(env *Env) mcp.ToolHandlerFor[validateBDIn, string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in validateBDIn) (*mcp.CallToolResult, string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, "", err
		}
		res, err := s.Exec(ctx, "validate_bd_design -quiet", 120*time.Second)
		if err != nil {
			return nil, "", err
		}
		if !res.OK {
			return nil, "", fmt.Errorf("validate_bd: %s", res.Error)
		}
		return nil, res.Value, nil
	}
}

func makeBDWrapper(env *Env) mcp.ToolHandlerFor[makeBDWrapperIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in makeBDWrapperIn) (*mcp.CallToolResult, bool, error) {
		if in.BD == "" {
			return nil, false, fmt.Errorf("bd name is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf(`
set wrapper [make_wrapper -files [get_files {%s.bd}] -top]
add_files -norecurse $wrapper
set_property top {%s_wrapper} [current_fileset]
`, in.BD, in.BD)
		res, err := s.Exec(ctx, script, 60*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("make_bd_wrapper: %s", res.Error)
		}
		return nil, true, nil
	}
}

// ── Checkpoints toolset ────────────────────────────────────────────────────

func RegisterCheckpointsToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "write_checkpoint",
		Description: "Save the current design checkpoint (write_checkpoint).",
	}, writeCheckpoint(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "read_checkpoint",
		Description: "Open a design checkpoint (read_checkpoint / open_checkpoint).",
	}, readCheckpoint(env))
}

type writeCheckpointIn struct {
	Path    string `json:"path"    jsonschema:"output .dcp file path"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type readCheckpointIn struct {
	Path    string `json:"path"    jsonschema:"path to .dcp file"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func writeCheckpoint(env *Env) mcp.ToolHandlerFor[writeCheckpointIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in writeCheckpointIn) (*mcp.CallToolResult, bool, error) {
		if in.Path == "" {
			return nil, false, fmt.Errorf("path is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("write_checkpoint -force {%s}", in.Path)
		res, err := s.Exec(ctx, script, 120*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("write_checkpoint: %s", res.Error)
		}
		return nil, true, nil
	}
}

func readCheckpoint(env *Env) mcp.ToolHandlerFor[readCheckpointIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in readCheckpointIn) (*mcp.CallToolResult, bool, error) {
		if in.Path == "" {
			return nil, false, fmt.Errorf("path is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("open_checkpoint {%s}", in.Path)
		res, err := s.Exec(ctx, script, 120*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("read_checkpoint: %s", res.Error)
		}
		return nil, true, nil
	}
}

// ── Hardware toolset ───────────────────────────────────────────────────────

func RegisterHardwareToolset(s *mcp.Server, env *Env) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_open",
		Description: "Open the hardware manager and connect to a hardware server.",
	}, hwOpen(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_program",
		Description: "Program a device with a bitstream.",
	}, hwProgram(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_list_devices",
		Description: "List connected hardware devices.",
	}, hwListDevices(env))

	mcp.AddTool(s, &mcp.Tool{
		Name:        "hw_close",
		Description: "Close the hardware manager connection.",
	}, hwClose(env))
}

type hwOpenIn struct {
	Host    string `json:"host"    jsonschema:"hardware server host, default localhost"`
	Port    int    `json:"port"    jsonschema:"hardware server port, default 3121"`
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type hwProgramIn struct {
	Bitstream string `json:"bitstream" jsonschema:"path to .bit file"`
	Device    string `json:"device"    jsonschema:"device name (optional, uses first device)"`
	Session   string `json:"session"   jsonschema:"session name, default 'main'"`
}
type hwListDevicesIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}
type hwCloseIn struct {
	Session string `json:"session" jsonschema:"session name, default 'main'"`
}

func hwOpen(env *Env) mcp.ToolHandlerFor[hwOpenIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwOpenIn) (*mcp.CallToolResult, bool, error) {
		host := in.Host
		if host == "" {
			host = "localhost"
		}
		port := in.Port
		if port == 0 {
			port = 3121
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := fmt.Sprintf("open_hw_manager; connect_hw_server -host {%s} -port %d", host, port)
		res, err := s.Exec(ctx, script, 30*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("hw_open: %s", res.Error)
		}
		return nil, true, nil
	}
}

func hwProgram(env *Env) mcp.ToolHandlerFor[hwProgramIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwProgramIn) (*mcp.CallToolResult, bool, error) {
		if in.Bitstream == "" {
			return nil, false, fmt.Errorf("bitstream path is required")
		}
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		var deviceExpr string
		if in.Device != "" {
			deviceExpr = fmt.Sprintf("[get_hw_devices {%s}]", in.Device)
		} else {
			deviceExpr = "[lindex [get_hw_devices] 0]"
		}
		script := fmt.Sprintf(`
set dev %s
current_hw_device $dev
set_property PROGRAM.FILE {%s} $dev
program_hw_devices $dev
refresh_hw_device $dev
`, deviceExpr, in.Bitstream)
		res, err := s.Exec(ctx, script, 120*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("hw_program: %s", res.Error)
		}
		return nil, true, nil
	}
}

func hwListDevices(env *Env) mcp.ToolHandlerFor[hwListDevicesIn, []string] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwListDevicesIn) (*mcp.CallToolResult, []string, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, nil, err
		}
		res, err := s.Exec(ctx, "get_hw_devices", 15*time.Second)
		if err != nil {
			return nil, nil, err
		}
		if !res.OK {
			return nil, nil, fmt.Errorf("hw_list_devices: %s", res.Error)
		}
		return nil, splitTclList(res.Value), nil
	}
}

func hwClose(env *Env) mcp.ToolHandlerFor[hwCloseIn, bool] {
	return func(ctx context.Context, req *mcp.CallToolRequest, in hwCloseIn) (*mcp.CallToolResult, bool, error) {
		s, err := MustSession(env, in.Session)
		if err != nil {
			return nil, false, err
		}
		script := "disconnect_hw_server; close_hw_manager"
		res, err := s.Exec(ctx, script, 15*time.Second)
		if err != nil {
			return nil, false, err
		}
		if !res.OK {
			return nil, false, fmt.Errorf("hw_close: %s", res.Error)
		}
		return nil, true, nil
	}
}
