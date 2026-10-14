// Package observability implements the JSONL call log and stats.
package observability

import (
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// CallEntry is one record in the JSONL call log.
type CallEntry struct {
	Timestamp  time.Time     `json:"ts"`
	Tool       string        `json:"tool"`
	Session    string        `json:"session,omitempty"`
	JobID      string        `json:"job_id,omitempty"`
	Duration   time.Duration `json:"duration_ns"`
	Status     string        `json:"status"` // "ok" | "error"
	Error      string        `json:"error,omitempty"`
}

// Logger writes JSONL call records to a file and keeps in-memory stats.
type Logger struct {
	mu      sync.Mutex
	f       *os.File
	recent  []CallEntry
	maxRecent int

	// per-tool call counts
	counts  map[string]*atomic.Int64
	errors  map[string]*atomic.Int64
	countMu sync.RWMutex

	startedAt time.Time
}

// New creates a Logger writing to logPath.
func New(logPath string, maxRecent int) (*Logger, error) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	if maxRecent <= 0 {
		maxRecent = 200
	}
	return &Logger{
		f:         f,
		maxRecent: maxRecent,
		counts:    make(map[string]*atomic.Int64),
		errors:    make(map[string]*atomic.Int64),
		startedAt: time.Now(),
	}, nil
}

// Record writes one call entry.
func (l *Logger) Record(e CallEntry) {
	// Update per-tool counters
	l.countMu.Lock()
	if _, ok := l.counts[e.Tool]; !ok {
		l.counts[e.Tool] = &atomic.Int64{}
		l.errors[e.Tool] = &atomic.Int64{}
	}
	cnt := l.counts[e.Tool]
	errcnt := l.errors[e.Tool]
	l.countMu.Unlock()

	cnt.Add(1)
	if e.Status == "error" {
		errcnt.Add(1)
	}

	// Write JSONL
	data, _ := json.Marshal(e)
	l.mu.Lock()
	_, _ = l.f.Write(append(data, '\n'))

	// Keep ring buffer of recent calls
	l.recent = append(l.recent, e)
	if len(l.recent) > l.maxRecent {
		l.recent = l.recent[len(l.recent)-l.maxRecent:]
	}
	l.mu.Unlock()
}

// Recent returns up to n recent entries, optionally filtered by tool name.
func (l *Logger) Recent(n int, toolFilter string) []CallEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []CallEntry
	for i := len(l.recent) - 1; i >= 0 && len(out) < n; i-- {
		e := l.recent[i]
		if toolFilter != "" && e.Tool != toolFilter {
			continue
		}
		out = append([]CallEntry{e}, out...)
	}
	return out
}

// ToolStats returns per-tool call/error counts.
type ToolStat struct {
	Tool   string `json:"tool"`
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
}

// Stats returns aggregate server statistics.
type Stats struct {
	Uptime    time.Duration `json:"uptime_ns"`
	StartedAt time.Time     `json:"started_at"`
	Tools     []ToolStat    `json:"tools"`
}

func (l *Logger) Stats() Stats {
	l.countMu.RLock()
	defer l.countMu.RUnlock()

	tools := make([]ToolStat, 0, len(l.counts))
	for name, cnt := range l.counts {
		tools = append(tools, ToolStat{
			Tool:   name,
			Calls:  cnt.Load(),
			Errors: l.errors[name].Load(),
		})
	}
	return Stats{
		Uptime:    time.Since(l.startedAt),
		StartedAt: l.startedAt,
		Tools:     tools,
	}
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.f.Close()
}
