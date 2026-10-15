# vconnect

A Vivado MCP server. Lets an LLM agent drive the complete Vivado Design Suite over the [Model Context Protocol](https://modelcontextprotocol.io). Board and part agnostic — works with any Xilinx/AMD device.

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

## Tools

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
| `reports` | `timing_summary`, `utilization`, `power_report`, `drc_report`, `methodology_report`, `cdc_report`, `clock_interaction`, `critical_paths` |
| `constraints` | `read_xdc`, `append_xdc`, `replace_xdc_constraint`, `reload_xdc` |
| `ip` | `list_ip_catalog`, `create_ip`, `configure_ip`, `generate_ip`, `upgrade_ip` |
| `bd` | `create_bd`, `add_bd_cell`, `connect_bd_net`, `validate_bd`, `make_bd_wrapper` |
| `checkpoints` | `write_checkpoint`, `read_checkpoint` |
| `hardware` | `hw_open`, `hw_program`, `hw_list_devices`, `hw_close` |

---

## Requirements

- **Vivado** 2020.1 or later (any edition, including WebPACK)
- **Go** 1.25 or later (only needed to build from source)
- OS: Linux, Windows, or NixOS

---

## Installation

### Pre-built binary

Download the latest binary from the [Releases](https://github.com/jhush/vconnect/releases) page.

### Build from source

```bash
git clone https://github.com/jhush/vconnect
cd vconnect
make build
```

The binary is placed at `./vconnect` (or `vconnect.exe` on Windows).

For a fully static binary (no libc dependency):

```bash
make build-static   # produces vconnect-linux-amd64
```

---

## Platform Setup

### Linux

Ensure `vivado` is on your `PATH`, or set `VIVADO_BIN`:

```bash
export VIVADO_BIN=/tools/Xilinx/Vivado/2023.2/bin/vivado
```

Run the server:

```bash
./vconnect -vivado "$VIVADO_BIN" -runs-dir ./runs
```

### NixOS

Install Go via `nix-shell` or add it to your system packages:

```nix
# shell.nix
{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  buildInputs = [ pkgs.go ];
}
```

Then build and run as on Linux. Vivado itself is not packaged in nixpkgs — install it from the Xilinx installer and point `VIVADO_BIN` at the binary.

```bash
nix-shell --run "make build"
VIVADO_BIN=/opt/Xilinx/Vivado/2023.2/bin/vivado ./vconnect
```

### Windows

> Vivado must be installed and its `bin` directory on `PATH`, or pass `-vivado` explicitly.

Build in PowerShell or Git Bash (Go must be installed):

```powershell
git clone https://github.com/jhush/vconnect
cd vconnect
go build -o vconnect.exe ./cmd/vconnect/
```

Run:

```powershell
.\vconnect.exe -vivado "C:\Xilinx\Vivado\2023.2\bin\vivado.bat"
```

---

## MCP Client Setup

### Claude Code

Add the following to `~/.claude/mcp.json` (create it if it doesn't exist):

```json
{
  "mcpServers": {
    "vconnect": {
      "command": "/path/to/vconnect",
      "args": [
        "-vivado", "vivado",
        "-runs-dir", "/path/to/runs"
      ],
      "env": {
        "VCONNECT_BOOTSTRAP": "/path/to/vconnect/tcl/bootstrap.tcl"
      }
    }
  }
}
```

On Windows, use the full path to `vconnect.exe` and forward slashes or escaped backslashes.

A ready-to-edit `mcp.json` is included in the repository root.

---

## Usage Example

```
session_start {}
enable_toolset {"name": "target"}
list_boards {"filter": "nexys"}
enable_toolset {"name": "project"}
create_project {
  "name": "myproj",
  "dir": "/tmp/myproj",
  "board": "digilentinc.com:nexys4_ddr:part0:1.1"
}
enable_toolset {"name": "flow"}
launch_synth {}         → {"job_id": "1728999600-ab3c"}
run_status {"job_id": "1728999600-ab3c"}
enable_toolset {"name": "reports"}
timing_summary {}
```

---

## Configuration Flags

| Flag | Default | Description |
|---|---|---|
| `-vivado` | `$VIVADO_BIN` or `vivado` | Path to the Vivado executable |
| `-runs-dir` | `./runs` | Directory for job artifacts |
| `-log` | `./vconnect.log` | JSONL call log path |
| `-tmp-dir` | `$TMPDIR/vconnect` | Temp directory for per-call result files |
| `-version` | — | Print version and exit |

`VCONNECT_BOOTSTRAP` environment variable overrides the path to `tcl/bootstrap.tcl`.

---

