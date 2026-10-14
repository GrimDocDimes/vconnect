// Package jobmgr implements the long-running job manager.
//
// Long operations (synthesis, implementation, bitstream) get a job_id and
// a directory runs/<job_id>/ with:
//   - status.json  – current state, provenance, timestamps
//   - run.log      – raw Tcl/Vivado output
//   - reports/     – any report files
//
// State persists to disk so jobs survive server restarts.
package jobmgr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// JobStatus represents the lifecycle state of a job.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobDone      JobStatus = "done"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// Provenance captures reproducibility metadata for a run.
type Provenance struct {
	VivadoVersion string            `json:"vivado_version,omitempty"`
	Part          string            `json:"part,omitempty"`
	Board         string            `json:"board,omitempty"`
	SourceHashes  map[string]string `json:"source_hashes,omitempty"` // file -> sha256
	Strategy      string            `json:"strategy,omitempty"`
	Directives    map[string]string `json:"directives,omitempty"`
}

// StatusFile is the on-disk representation of a job.
type StatusFile struct {
	JobID       string      `json:"job_id"`
	Kind        string      `json:"kind"` // synth, impl, bitstream, tcl
	Status      JobStatus   `json:"status"`
	SessionName string      `json:"session_name"`
	TclScript   string      `json:"tcl_script,omitempty"`
	Provenance  Provenance  `json:"provenance,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	FinishedAt  *time.Time  `json:"finished_at,omitempty"`
	Error       string      `json:"error,omitempty"`
	LogFile     string      `json:"log_file"`
	ReportsDir  string      `json:"reports_dir"`
	Artifacts   []string    `json:"artifacts,omitempty"`
	Extra       interface{} `json:"extra,omitempty"`
}

// Job is an in-memory handle to a running/queued job.
type Job struct {
	mu     sync.RWMutex
	sf     StatusFile
	runsDir string

	cancelFn context.CancelFunc
	doneCh   chan struct{}
}

// ID returns the job's ID.
func (j *Job) ID() string {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.sf.JobID
}

// StatusFile returns a snapshot of the status.
func (j *Job) Snapshot() StatusFile {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.sf
}

// Cancel attempts to cancel the job.
func (j *Job) Cancel() {
	if j.cancelFn != nil {
		j.cancelFn()
	}
}

// Wait blocks until the job completes.
func (j *Job) Wait() {
	<-j.doneCh
}

// Manager owns the runs directory and all in-progress jobs.
type Manager struct {
	runsDir string

	mu   sync.Mutex
	jobs map[string]*Job
}

// New creates a Manager backed by runsDir (created if needed).
func New(runsDir string) (*Manager, error) {
	if err := os.MkdirAll(runsDir, 0o755); err != nil {
		return nil, fmt.Errorf("jobmgr: mkdir %s: %w", runsDir, err)
	}
	m := &Manager{
		runsDir: runsDir,
		jobs:    make(map[string]*Job),
	}
	// Reload persisted job states from disk (survived restart).
	_ = m.reloadFromDisk()
	return m, nil
}

func (m *Manager) reloadFromDisk() error {
	entries, err := os.ReadDir(m.runsDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sfPath := filepath.Join(m.runsDir, e.Name(), "status.json")
		data, err := os.ReadFile(sfPath)
		if err != nil {
			continue
		}
		var sf StatusFile
		if err := json.Unmarshal(data, &sf); err != nil {
			continue
		}
		// If job was running/pending when server died, mark it failed.
		if sf.Status == JobRunning || sf.Status == JobPending {
			sf.Status = JobFailed
			sf.Error = "server restarted while job was in-flight"
			now := time.Now()
			sf.FinishedAt = &now
			_ = writeStatusFile(sf)
		}
		j := &Job{sf: sf, runsDir: m.runsDir, doneCh: make(chan struct{})}
		close(j.doneCh) // already terminal
		m.jobs[sf.JobID] = j
	}
	return nil
}

// Submit creates a new job and immediately starts fn in a goroutine.
// fn receives a context (cancelled on Cancel()), a log file, and a reports dir.
func (m *Manager) Submit(kind string, sessionName string, prov Provenance, fn func(ctx context.Context, logPath, reportsDir string) error) (*Job, error) {
	id := newJobID()
	jobDir := filepath.Join(m.runsDir, id)
	if err := os.MkdirAll(filepath.Join(jobDir, "reports"), 0o755); err != nil {
		return nil, fmt.Errorf("jobmgr: mkdir job dir: %w", err)
	}

	logPath := filepath.Join(jobDir, "run.log")
	reportsDir := filepath.Join(jobDir, "reports")

	sf := StatusFile{
		JobID:       id,
		Kind:        kind,
		Status:      JobPending,
		SessionName: sessionName,
		Provenance:  prov,
		CreatedAt:   time.Now(),
		LogFile:     logPath,
		ReportsDir:  reportsDir,
	}
	if err := writeStatusFile(sf); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{
		sf:       sf,
		runsDir:  m.runsDir,
		cancelFn: cancel,
		doneCh:   make(chan struct{}),
	}

	m.mu.Lock()
	m.jobs[id] = j
	m.mu.Unlock()

	go func() {
		defer close(j.doneCh)
		defer cancel()

		j.mu.Lock()
		now := time.Now()
		j.sf.Status = JobRunning
		j.sf.StartedAt = &now
		sf := j.sf
		j.mu.Unlock()
		_ = writeStatusFile(sf)

		runErr := fn(ctx, logPath, reportsDir)

		j.mu.Lock()
		fin := time.Now()
		j.sf.FinishedAt = &fin
		if errors.Is(runErr, context.Canceled) {
			j.sf.Status = JobCancelled
		} else if runErr != nil {
			j.sf.Status = JobFailed
			j.sf.Error = runErr.Error()
		} else {
			j.sf.Status = JobDone
		}
		sf = j.sf
		j.mu.Unlock()
		_ = writeStatusFile(sf)
	}()

	return j, nil
}

// Get returns the job with the given ID, or an error.
func (m *Manager) Get(id string) (*Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q not found", id)
	}
	return j, nil
}

// List returns snapshots of all jobs.
func (m *Manager) List() []StatusFile {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]StatusFile, 0, len(m.jobs))
	for _, j := range m.jobs {
		out = append(out, j.Snapshot())
	}
	return out
}

func writeStatusFile(sf StatusFile) error {
	sfPath := filepath.Join(sf.LogFile[:len(sf.LogFile)-len("run.log")], "status.json")
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return err
	}
	tmp := sfPath + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, sfPath)
}

func newJobID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), string(b))
}
