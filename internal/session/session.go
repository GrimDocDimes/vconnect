// Package session manages long-lived Vivado -mode tcl child processes.
//
// Design:
//   - One command at a time per session (mutex-serialised).
//   - Session never writes to its own stdin except via Exec.
//   - Vivado stdout is read by a dedicated goroutine that looks only for
//     the sentinel line "VCONN_DONE <id>".  All other output is discarded
//     (Vivado leaks diagnostic traffic onto stdout).
//   - Results are exchanged via a per-call JSON temp file (atomic write
//     from Tcl side: write .part, rename).
//   - Messages (INFO/WARNING/ERROR with IDs) are parsed from stderr.
package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// State represents session lifecycle state.
type State int

const (
	StateIdle     State = iota // waiting for a command
	StateBusy                  // command in flight
	StateCrashed               // vivado exited unexpectedly
	StateStarting              // not yet ready
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateBusy:
		return "busy"
	case StateCrashed:
		return "crashed"
	case StateStarting:
		return "starting"
	}
	return "unknown"
}

// VivadoMessage is a parsed Vivado message stream entry.
type VivadoMessage struct {
	Severity string `json:"severity"` // INFO, WARNING, CRITICAL WARNING, ERROR
	ID       string `json:"id"`       // e.g. "Synth 8-327"
	Text     string `json:"text"`
}

// ExecResult is returned by Exec.
type ExecResult struct {
	OK       bool            `json:"ok"`
	Value    string          `json:"value,omitempty"`
	Error    string          `json:"error,omitempty"`
	ErrCode  string          `json:"errorCode,omitempty"`
	Messages []VivadoMessage `json:"messages,omitempty"`
	Duration time.Duration   `json:"duration"`
}

// Config controls how a session is started.
type Config struct {
	// VivadoBin is the path to the vivado executable.
	// If empty, $VIVADO_BIN or "vivado" from PATH is used.
	VivadoBin string

	// BootstrapTcl is the path to tcl/bootstrap.tcl embedded in the binary.
	BootstrapTcl string

	// TmpDir is the directory for per-call result files.
	// If empty, os.TempDir()/vconnect is used.
	TmpDir string

	// StartTimeout is how long to wait for the VCONN_READY sentinel.
	StartTimeout time.Duration

	// DefaultTimeout is the per-call timeout if none is supplied.
	DefaultTimeout time.Duration

	// Name is a human label (e.g. "main", "hardware").
	Name string
}

const defaultStartTimeout = 120 * time.Second
const defaultCallTimeout = 300 * time.Second

// Session holds a running Vivado process.
type Session struct {
	cfg  Config
	name string

	mu      sync.Mutex // serialises Exec calls
	state   State
	stateCV *sync.Cond

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr *stderrCollector

	// sentinelCh receives "VCONN_DONE <id>" lines from the stdout reader goroutine.
	sentinelCh chan string

	tmpDir  string
	callSeq uint64 // atomic

	startedAt time.Time
	crashErr  error
}

// New creates a new session and starts Vivado.
func New(ctx context.Context, cfg Config) (*Session, error) {
	if cfg.StartTimeout == 0 {
		cfg.StartTimeout = defaultStartTimeout
	}
	if cfg.DefaultTimeout == 0 {
		cfg.DefaultTimeout = defaultCallTimeout
	}
	if cfg.Name == "" {
		cfg.Name = "main"
	}

	bin := cfg.VivadoBin
	if bin == "" {
		bin = os.Getenv("VIVADO_BIN")
	}
	if bin == "" {
		bin = "vivado"
	}

	tmpDir := cfg.TmpDir
	if tmpDir == "" {
		tmpDir = filepath.Join(os.TempDir(), "vconnect", cfg.Name)
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return nil, fmt.Errorf("session: mkdir tmpdir: %w", err)
	}

	s := &Session{
		cfg:        cfg,
		name:       cfg.Name,
		state:      StateStarting,
		sentinelCh: make(chan string, 8),
		tmpDir:     tmpDir,
		startedAt:  time.Now(),
	}
	s.stateCV = sync.NewCond(&s.mu)

	// Build command: vivado -mode tcl -nojournal -nolog
	args := []string{"-mode", "tcl", "-nojournal", "-nolog"}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ()) // inherit full environment

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("session: stdin pipe: %w", err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("session: stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("session: stderr pipe: %w", err)
	}

	s.cmd = cmd
	s.stdin = stdin
	s.stderr = newStderrCollector(stderrPipe)

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("session: start vivado: %w", err)
	}

	// Start stdout sentinel reader.
	go s.readStdout(stdoutPipe)
	// Start crash monitor.
	go s.watchCrash()

	// Load bootstrap.
	if err := s.loadBootstrap(cfg.StartTimeout); err != nil {
		_ = s.Kill()
		return nil, fmt.Errorf("session: bootstrap: %w", err)
	}

	s.mu.Lock()
	s.state = StateIdle
	s.stateCV.Broadcast()
	s.mu.Unlock()

	return s, nil
}

// loadBootstrap sources the bootstrap Tcl file and waits for VCONN_READY.
func (s *Session) loadBootstrap(timeout time.Duration) error {
	if s.cfg.BootstrapTcl == "" {
		return errors.New("BootstrapTcl path not set")
	}
	// source the file
	cmd := fmt.Sprintf("source {%s}\n", s.cfg.BootstrapTcl)
	if _, err := io.WriteString(s.stdin, cmd); err != nil {
		return fmt.Errorf("write source cmd: %w", err)
	}

	deadline := time.After(timeout)
	for {
		select {
		case line := <-s.sentinelCh:
			if line == "VCONN_READY" {
				return nil
			}
			// could be intermediate output; keep waiting
		case <-deadline:
			return fmt.Errorf("timed out waiting for VCONN_READY after %v", timeout)
		}
	}
}

// readStdout consumes Vivado's stdout line-by-line.
// It only cares about lines matching the sentinel pattern or VCONN_READY.
// Everything else is discarded (Vivado leaks INFO messages to stdout).
func (s *Session) readStdout(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if line == "VCONN_READY" || strings.HasPrefix(line, "VCONN_DONE ") {
			s.sentinelCh <- line
		}
	}
	// EOF means Vivado exited — this is handled by watchCrash.
}

// watchCrash monitors the process and marks the session crashed.
func (s *Session) watchCrash() {
	err := s.cmd.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateStarting && s.state != StateCrashed {
		if err != nil {
			s.crashErr = err
		} else {
			s.crashErr = errors.New("vivado exited unexpectedly")
		}
		s.state = StateCrashed
		s.stateCV.Broadcast()
	}
}

// stderrCollector buffers stderr and parses Vivado messages.
type stderrCollector struct {
	mu       sync.Mutex
	lines    []string
	messages []VivadoMessage
}

func newStderrCollector(r io.Reader) *stderrCollector {
	sc := &stderrCollector{}
	go sc.read(r)
	return sc
}

func (sc *stderrCollector) read(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		sc.mu.Lock()
		sc.lines = append(sc.lines, line)
		if m, ok := parseVivadoMessage(line); ok {
			sc.messages = append(sc.messages, m)
		}
		sc.mu.Unlock()
	}
}

func (sc *stderrCollector) drain() []VivadoMessage {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	msgs := sc.messages
	sc.messages = nil
	return msgs
}

// parseVivadoMessage attempts to parse a Vivado log line.
// Format: INFO: [Synth 8-327] text
func parseVivadoMessage(line string) (VivadoMessage, bool) {
	for _, sev := range []string{"CRITICAL WARNING", "ERROR", "WARNING", "INFO"} {
		prefix := sev + ": ["
		if strings.HasPrefix(line, prefix) {
			rest := line[len(prefix):]
			idx := strings.Index(rest, "] ")
			if idx < 0 {
				continue
			}
			return VivadoMessage{
				Severity: sev,
				ID:       rest[:idx],
				Text:     rest[idx+2:],
			}, true
		}
	}
	return VivadoMessage{}, false
}

// Exec sends a Tcl script to Vivado and waits for the result.
// Only one Exec may run at a time per session.
func (s *Session) Exec(ctx context.Context, script string, timeout time.Duration) (*ExecResult, error) {
	if timeout == 0 {
		timeout = s.cfg.DefaultTimeout
	}

	s.mu.Lock()
	for s.state == StateBusy || s.state == StateStarting {
		s.stateCV.Wait()
	}
	if s.state == StateCrashed {
		s.mu.Unlock()
		return nil, fmt.Errorf("session %q has crashed: %v", s.name, s.crashErr)
	}
	s.state = StateBusy
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.state == StateBusy {
			s.state = StateIdle
		}
		s.stateCV.Broadcast()
		s.mu.Unlock()
	}()

	id := fmt.Sprintf("%d", atomic.AddUint64(&s.callSeq, 1))
	tmpFile := filepath.Join(s.tmpDir, "result_"+id+".json")

	// Build the Tcl call
	escapedScript := strings.ReplaceAll(script, `\`, `\\`)
	tclCmd := fmt.Sprintf("vconn_exec %s {%s} {%s}\n", id, tmpFile, escapedScript)

	start := time.Now()

	// drain stale messages before call
	_ = s.stderr.drain()

	if _, err := io.WriteString(s.stdin, tclCmd); err != nil {
		return nil, fmt.Errorf("session: write to vivado: %w", err)
	}

	// Wait for sentinel
	deadline := time.After(timeout)
	for {
		select {
		case line := <-s.sentinelCh:
			if line == "VCONN_DONE "+id {
				goto done
			}
			// other sentinel (shouldn't happen with one-at-a-time, but be safe)

		case <-deadline:
			return nil, fmt.Errorf("session: timeout after %v waiting for VCONN_DONE %s", timeout, id)

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

done:
	elapsed := time.Since(start)
	msgs := s.stderr.drain()

	// Read temp file
	data, err := os.ReadFile(tmpFile)
	_ = os.Remove(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("session: read result file: %w", err)
	}

	var res ExecResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("session: unmarshal result: %w (raw: %s)", err, string(data))
	}
	res.Duration = elapsed
	res.Messages = msgs
	return &res, nil
}

// State returns the current session state without blocking.
func (s *Session) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// Name returns the session's name.
func (s *Session) Name() string { return s.name }

// StartedAt returns when the session process was launched.
func (s *Session) StartedAt() time.Time { return s.startedAt }

// Kill terminates the Vivado process immediately.
func (s *Session) Kill() error {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Kill()
	}
	return nil
}

// Restart kills and restarts the session. State is LOST.
// Callers should warn that in-memory Vivado state is discarded.
func (s *Session) Restart(ctx context.Context) error {
	_ = s.Kill()
	// brief pause to let the process die
	time.Sleep(500 * time.Millisecond)

	fresh, err := New(ctx, s.cfg)
	if err != nil {
		return err
	}
	// swap internals
	s.mu.Lock()
	s.cmd = fresh.cmd
	s.stdin = fresh.stdin
	s.stderr = fresh.stderr
	s.sentinelCh = fresh.sentinelCh
	s.tmpDir = fresh.tmpDir
	s.state = StateIdle
	s.crashErr = nil
	s.startedAt = time.Now()
	s.stateCV.Broadcast()
	s.mu.Unlock()
	return nil
}

// VivadoVersion asks Vivado for its version string.
func (s *Session) VivadoVersion(ctx context.Context) (string, error) {
	res, err := s.Exec(ctx, "vconn_version", 10*time.Second)
	if err != nil {
		return "", err
	}
	if !res.OK {
		return "", fmt.Errorf("vivado: %s", res.Error)
	}
	return res.Value, nil
}

// ParseVivadoMessageExported is a test-exported wrapper around parseVivadoMessage.
func ParseVivadoMessageExported(line string) (VivadoMessage, bool) {
	return parseVivadoMessage(line)
}
