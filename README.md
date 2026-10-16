# vconnect

A Vivado MCP server. Lets an LLM agent drive the complete Vivado Design Suite over the [Model Context Protocol](https://modelcontextprotocol.io). Supports both project-mode and non-project (script) flows. Board and part agnostic.

---

## How It Works

vconnect spawns `vivado -mode tcl` as a long-lived child process and communicates with it over stdin/stdout using a sentinel + temp-file protocol:

```
LLM agent  ──MCP──►  vconnect  ──stdin──►  vivado -mode tcl
                                ◄──stdout──  "VCONN_DONE <id>"
                                ◄──file────  result_<id>.json
```

1. vconnect writes a Tcl call to Vivado's stdin.
2. Vivado executes it under `[catch]`, writes a JSON result file atomically, then prints a sentinel line.
3. vconnect reads the sentinel, reads the result file, and returns a structured response to the agent.

Vivado stdout carries diagnostic noise — vconnect ignores everything except the sentinel. Stderr is parsed for structured `INFO`/`WARNING`/`ERROR` messages (with IDs like `[Synth 8-327]`).

Long operations (synthesis, implementation, bitstream) run as async **jobs** with a `job_id`. Status persists to `runs/<job_id>/status.json` and survives server restarts.

---

## Tools — 101 total across 14 toolsets

### Starter Kit — always available

| Tool | Description |
|---|---|
| `session_start` | Start a named Vivado session |
| `session_status` | Check session state (`idle` / `busy` / `crashed`) |
| `run_tcl` | Execute arbitrary Tcl — full coverage escape hatch |
| `tcl_help` | Run `help <cmd>` inside Vivado |
| `list_toolsets` | List available on-demand toolsets |
| `enable_toolset` | Load a toolset by name |
| `server_stats` | Uptime, sessions, active jobs, call rates |
| `get_recent_calls` | Recent call log with optional tool filter |

### On-Demand Toolsets

Enable with `enable_toolset {"name": "<toolset>"}`:

| Toolset | Tools |
|---|---|
| `target` | `list_parts`, `list_boards`, `set_part`, `set_board` |
| `project` | `create_project`, `open_project`, `close_project`, `add_sources`, `set_top`, `update_compile_order`, `list_files`, `export_project_tcl` |
| `flow` | `launch_synth`, `launch_impl`, `launch_bitstream`, `set_strategy`, `run_status`, `reset_run`, `cancel_run` |
| `nonproject` | `read_verilog`, `read_vhdl`, `read_xdc_file`, `synth_design`, `opt_design`, `place_design`, `phys_opt_design`, `route_design`, `write_bitstream`, `link_design` |
| `reports` | `timing_summary`, `utilization`, `power_report`, `drc_report`, `methodology_report`, `cdc_report`, `clock_interaction`, `critical_paths` |
| `constraints` | `read_xdc`, `append_xdc`, `replace_xdc_constraint`, `reload_xdc` |
| `timing_constraints` | `create_clock`, `create_generated_clock`, `set_input_delay`, `set_output_delay`, `set_false_path`, `set_multicycle_path`, `set_max_delay`, `check_timing`, `get_clocks` |
| `design` | `get_cells`, `get_nets`, `get_ports`, `get_property`, `set_property`, `report_io`, `report_exceptions`, `write_verilog`, `write_vhdl`, `get_runs`, `create_pblock` |
| `ip` | `list_ip_catalog`, `create_ip`, `configure_ip`, `generate_ip`, `upgrade_ip` |
| `bd` | `create_bd`, `add_bd_cell`, `connect_bd_net`, `validate_bd`, `make_bd_wrapper` |
| `checkpoints` | `write_checkpoint`, `read_checkpoint` |
| `sim` | `launch_sim`, `run_sim`, `restart_sim`, `close_sim`, `add_wave`, `log_wave`, `set_sim_target` |
| `debug` | `create_debug_core`, `set_debug_core_property`, `connect_debug_port`, `implement_debug_core`, `hw_ila_run`, `hw_ila_wait`, `hw_vio_get`, `hw_vio_set`, `write_debug_probes` |
| `hardware` | `hw_open`, `hw_program`, `hw_list_devices`, `hw_close` |

---

## Requirements

- **Vivado** 2019.1 or later (any edition, including WebPACK)
- **Go** 1.22 or later (only needed to build from source)
- OS: Linux, NixOS, or Windows

---

## Installation

### Pre-built binary

Download the latest binary from the [Releases](https://github.com/GrimDocDimes/vconnect/releases) page and place it anywhere on your `PATH`.

### Build from source

```bash
git clone https://github.com/GrimDocDimes/vconnect
cd vconnect
make build
```

The binary is placed at `./vconnect`.

For a fully static binary with no libc dependency:

```bash
make build-static   # produces vconnect-linux-amd64
```

---

## Platform Setup

### Linux

Ensure `vivado` is on your `PATH`, or set `VIVADO_BIN`:

```bash
export VIVADO_BIN=/tools/Xilinx/Vivado/2023.2/bin/vivado
./vconnect -vivado "$VIVADO_BIN"
```

### NixOS

Vivado requires a FHS-compatible environment on NixOS. A ready-made `shell.nix` is included that sets up the correct library environment and launches vconnect automatically.

**One-time setup:**

1. Build the binary:
   ```bash
   cd /path/to/vconnect && make build
   ```

2. Edit `shell.nix` to point to your Vivado installation (the default path is `/run/media/jhush/OLDTING/Xilinx/Vivado/2019.2`).

3. Run directly:
   ```bash
   nix-shell /path/to/vconnect/shell.nix
   ```
   This drops into the FHS environment and starts vconnect.

For the MCP client, set the command to `nix-shell /path/to/vconnect/shell.nix` — it starts vconnect automatically.

### Windows

Vivado must be installed and its `bin` directory on `PATH`, or pass `-vivado` explicitly.

```powershell
git clone https://github.com/GrimDocDimes/vconnect
cd vconnect
go build -o vconnect.exe ./cmd/vconnect/
.\vconnect.exe -vivado "C:\Xilinx\Vivado\2023.2\bin\vivado.bat"
```

---

## MCP Client Setup

### Claude Code / any MCP client

Add to your MCP client config (e.g. `~/.claude/mcp.json`):

**Linux:**
```json
{
  "mcpServers": {
    "vconnect": {
      "command": "/path/to/vconnect",
      "args": ["-vivado", "vivado", "-runs-dir", "/path/to/runs"]
    }
  }
}
```

**NixOS (uses the included `shell.nix`):**
```json
{
  "mcpServers": {
    "vconnect": {
      "command": "nix-shell",
      "args": ["/path/to/vconnect/shell.nix"]
    }
  }
}
```

A ready-to-edit `mcp.json` is included in the repository root.

### Testing without a paid API (MCP Inspector)

```bash
# Linux/NixOS
npx @modelcontextprotocol/inspector ./vconnect -vivado vivado

# NixOS via shell.nix
npx @modelcontextprotocol/inspector nix-shell /path/to/vconnect/shell.nix
```

Opens `http://localhost:5173` — a web UI to call tools and inspect responses directly, no LLM API needed.

---

## Usage Example

```
session_start {}
enable_toolset {"name": "target"}
list_boards {"filter": "nexys"}

enable_toolset {"name": "project"}
create_project {"name": "myproj", "dir": "/tmp/myproj", "board": "digilentinc.com:nexys4_ddr:part0:1.1"}

enable_toolset {"name": "timing_constraints"}
create_clock {"name": "sys_clk", "period": 10.0, "port": "clk"}

enable_toolset {"name": "flow"}
launch_synth {}          → {"job_id": "..."}
run_status {"job_id": "..."}

enable_toolset {"name": "reports"}
timing_summary {}

enable_toolset {"name": "hardware"}
hw_open {}
hw_program {"bitstream": "/tmp/myproj/myproj.runs/impl_1/top.bit"}
```

**Non-project (script) mode:**
```
session_start {}
enable_toolset {"name": "nonproject"}
read_verilog {"files": ["/src/top.v", "/src/fifo.v"]}
read_xdc_file {"path": "/src/constraints.xdc"}
synth_design {"top": "top", "part": "xc7a35tcpg236-1"}
opt_design {}
place_design {}
route_design {}
write_bitstream {"file": "/out/top.bit", "force": true}
```

---

## Configuration Flags

| Flag | Default | Description |
|---|---|---|
| `-vivado` | `$VIVADO_BIN` or `vivado` | Path to the Vivado executable |
| `-runs-dir` | `./runs` | Directory for async job artifacts |
| `-log` | `./vconnect.log` | JSONL call log path |
| `-tmp-dir` | `$TMPDIR/vconnect` | Temp directory for per-call result files |
| `-version` | — | Print version and exit |

`VCONNECT_BOOTSTRAP` — override path to `tcl/bootstrap.tcl` (useful when binary and Tcl file are in separate locations).

---

## License

MIT
