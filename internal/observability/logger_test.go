package observability_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jhush/vconnect/internal/observability"
)

func TestLoggerRecordAndRecent(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "calls.jsonl")

	logger, err := observability.New(logPath, 50)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer logger.Close()

	// Record some entries
	for i := 0; i < 10; i++ {
		tool := "run_tcl"
		if i%3 == 0 {
			tool = "list_parts"
		}
		logger.Record(observability.CallEntry{
			Timestamp: time.Now(),
			Tool:      tool,
			Duration:  time.Duration(i) * time.Millisecond,
			Status:    "ok",
		})
	}

	// Retrieve all
	all := logger.Recent(100, "")
	if len(all) != 10 {
		t.Errorf("Recent(100,'') = %d entries, want 10", len(all))
	}

	// Filter by tool
	tclEntries := logger.Recent(100, "run_tcl")
	if len(tclEntries) == 0 {
		t.Errorf("expected some run_tcl entries")
	}
	for _, e := range tclEntries {
		if e.Tool != "run_tcl" {
			t.Errorf("filter failed: got tool %q", e.Tool)
		}
	}

	// Stats
	stats := logger.Stats()
	if stats.Uptime == 0 {
		t.Errorf("uptime should be > 0")
	}
	found := false
	for _, ts := range stats.Tools {
		if ts.Tool == "run_tcl" {
			found = true
			if ts.Calls == 0 {
				t.Errorf("run_tcl call count should be > 0")
			}
		}
	}
	if !found {
		t.Errorf("run_tcl not found in stats")
	}

	// Log file should be non-empty JSONL
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) == 0 {
		t.Errorf("log file is empty")
	}
}

func TestLoggerRingBuffer(t *testing.T) {
	dir := t.TempDir()
	logger, err := observability.New(filepath.Join(dir, "calls.jsonl"), 5) // tiny ring
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	for i := 0; i < 10; i++ {
		logger.Record(observability.CallEntry{
			Timestamp: time.Now(),
			Tool:      "tool",
			Status:    "ok",
		})
	}

	recent := logger.Recent(100, "")
	if len(recent) != 5 {
		t.Errorf("ring buffer: got %d want 5", len(recent))
	}
}
