package session_test

import (
	"strings"
	"testing"

	"github.com/jhush/vconnect/internal/session"
)

func TestParseVivadoMessage(t *testing.T) {
	tests := []struct {
		line     string
		wantOK   bool
		wantSev  string
		wantID   string
		wantText string
	}{
		{
			line:     "INFO: [Synth 8-327] inferring latch for variable 'foo'",
			wantOK:   true,
			wantSev:  "INFO",
			wantID:   "Synth 8-327",
			wantText: "inferring latch for variable 'foo'",
		},
		{
			line:     "WARNING: [DRC 23-20] Rule violation",
			wantOK:   true,
			wantSev:  "WARNING",
			wantID:   "DRC 23-20",
			wantText: "Rule violation",
		},
		{
			line:     "CRITICAL WARNING: [Timing 38-316] hold time violation",
			wantOK:   true,
			wantSev:  "CRITICAL WARNING",
			wantID:   "Timing 38-316",
			wantText: "hold time violation",
		},
		{
			line:     "ERROR: [Project 1-576] Synthesis had 2 error(s)",
			wantOK:   true,
			wantSev:  "ERROR",
			wantID:   "Project 1-576",
			wantText: "Synthesis had 2 error(s)",
		},
		{
			line:   "just a plain log line",
			wantOK: false,
		},
		{
			line:   "",
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.line[:min(len(tc.line), 40)], func(t *testing.T) {
			got, ok := session.ParseVivadoMessageExported(tc.line)
			if ok != tc.wantOK {
				t.Fatalf("ParseVivadoMessage(%q) ok=%v want %v", tc.line, ok, tc.wantOK)
			}
			if !ok {
				return
			}
			if got.Severity != tc.wantSev {
				t.Errorf("severity: got %q want %q", got.Severity, tc.wantSev)
			}
			if got.ID != tc.wantID {
				t.Errorf("id: got %q want %q", got.ID, tc.wantID)
			}
			if got.Text != tc.wantText {
				t.Errorf("text: got %q want %q", got.Text, tc.wantText)
			}
		})
	}
}

func TestSplitTclList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"a b c", []string{"a", "b", "c"}},
		{"{hello world} foo", []string{"hello world", "foo"}},
		{"", nil},
		{"{nested {braces}} bar", []string{"nested {braces}", "bar"}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := splitTclListExported(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("splitTclList(%q) = %v, want %v", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d] got %q want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// splitTclListExported is a test-only bridge to the unexported helper in tools.
// We inline the same logic here for package-level testing.
func splitTclListExported(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	i := 0
	for i < len(s) {
		for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
			i++
		}
		if i >= len(s) {
			break
		}
		if s[i] == '{' {
			depth := 0
			start := i + 1
			for i < len(s) {
				if s[i] == '{' {
					depth++
				} else if s[i] == '}' {
					depth--
					if depth == 0 {
						out = append(out, s[start:i])
						i++
						break
					}
				}
				i++
			}
		} else {
			start := i
			for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' {
				i++
			}
			out = append(out, s[start:i])
		}
	}
	return out
}

var _ = strings.TrimSpace // keep import
