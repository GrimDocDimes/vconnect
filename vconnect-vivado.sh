#!/usr/bin/env bash
# vconnect-vivado.sh — runs vconnect inside the Vivado 2019.2 FHS environment.
# MCP clients (Claude Code) should point to this script as the server command.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VIVADO_DIR="/run/media/jhush/OLDTING/Xilinx/Vivado/2019.2"
VIVADO_BIN="${VIVADO_DIR}/bin/vivado"
SHELL_NIX="${VIVADO_DIR}/bin/shell.nix"
BOOTSTRAP="${SCRIPT_DIR}/tcl/bootstrap.tcl"
RUNS_DIR="${SCRIPT_DIR}/runs"
LOG_FILE="${SCRIPT_DIR}/vconnect.log"

mkdir -p "$RUNS_DIR"

exec nix-shell "$SHELL_NIX" \
    --run "
        export PATH='${VIVADO_DIR}/bin':\$PATH
        export LIBRARY_PATH=/usr/lib
        export LD_LIBRARY_PATH=/usr/lib:/usr/lib64
        export VCONNECT_BOOTSTRAP='${BOOTSTRAP}'
        exec '${SCRIPT_DIR}/vconnect' \
            -vivado '${VIVADO_BIN}' \
            -runs-dir '${RUNS_DIR}' \
            -log '${LOG_FILE}'
    "
