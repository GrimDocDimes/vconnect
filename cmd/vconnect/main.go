// vconnect — Board-Agnostic Vivado MCP Server
//
// Usage:
//
//	vconnect [flags]
//
// Flags:
//
//	-vivado      path to vivado binary (default: $VIVADO_BIN or "vivado")
//	-runs-dir    directory for job run artifacts (default: ./runs)
//	-log         JSONL call log path (default: vconnect.log)
//	-tmp-dir     temp directory for per-call result files (default: $TMPDIR/vconnect)
//	-version     print version and exit
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"syscall"
                                                   
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/jhush/vconnect/internal/jobmgr"
	"github.com/jhush/vconnect/internal/observability"
	"github.com/jhush/vconnect/internal/router"
	"github.com/jhush/vconnect/internal/tools"
)

// version is set by goreleaser / ldflags.
var version = "dev"

func main() {
	var (
		vivadoBin = flag.String("vivado", "", "path to vivado binary (default: $VIVADO_BIN or vivado in PATH)")
		runsDir   = flag.String("runs-dir", "runs", "directory for job run artifacts")
		logPath   = flag.String("log", "vconnect.log", "JSONL call log path")
		tmpDir    = flag.String("tmp-dir", "", "temp directory for per-call result files")
		showVer   = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		if bi, ok := debug.ReadBuildInfo(); ok {
			fmt.Printf("vconnect %s (go %s)\n", version, bi.GoVersion)
		} else {
			fmt.Printf("vconnect %s\n", version)
		}
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Resolve bootstrap Tcl path (embedded at build time or next to binary).
	bootstrapTcl := findBootstrapTcl()

	// Create observability logger.
	obs, err := observability.New(*logPath, 500)
	if err != nil {
		slog.Error("failed to open log file", "path", *logPath, "err", err)
		os.Exit(1)
	}
	defer obs.Close()

	// Create job manager.
	jobs, err := jobmgr.New(*runsDir)
	if err != nil {
		slog.Error("failed to create job manager", "runs-dir", *runsDir, "err", err)
		os.Exit(1)
	}

	// Build Env (shared dependency bundle).
	env := &tools.Env{
		Sessions: tools.NewRegistry(),
		Jobs:     jobs,
		Log:      obs,
		Config: &tools.Config{
			VivadoBin:    *vivadoBin,
			BootstrapTcl: bootstrapTcl,
			TmpDir:       *tmpDir,
			RunsDir:      *runsDir,
			LogPath:      *logPath,
		},
	}

	// Create MCP server.
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "vconnect",
		Version: version,
	}, nil)

	// Register starter kit (always available).
	tools.RegisterStarterKit(srv, env)

	// Build toolset registry.
	reg := router.New(srv, nil) // deps unused — env is captured in closures
	env.Registry = reg
	tools.RegisterAllToolsets(reg, srv, env)

	slog.Info("vconnect starting", "version", version, "bootstrap", bootstrapTcl)

	// Handle OS signals for clean shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Run MCP server over stdio (Claude Code / Codex CLI connect via stdio).
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		if ctx.Err() == nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}
	slog.Info("vconnect stopped")
}

// findBootstrapTcl locates tcl/bootstrap.tcl relative to the binary or
// working directory. In release builds this is embedded; here we search
// at runtime.
func findBootstrapTcl() string {
	candidates := []string{
		"tcl/bootstrap.tcl",
		filepath.Join(filepath.Dir(os.Args[0]), "tcl", "bootstrap.tcl"),
		"/usr/share/vconnect/bootstrap.tcl",
	}
	// Allow override via environment.
	if env := os.Getenv("VCONNECT_BOOTSTRAP"); env != "" {
		candidates = append([]string{env}, candidates...)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	// Return default; session.New will error clearly if not found.
	return "tcl/bootstrap.tcl"
}
