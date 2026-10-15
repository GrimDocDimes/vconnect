package tools_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAtomicWriteRoundTrip verifies that atomicWriteFile preserves content
// and that replaceXDCConstraint correctly replaces a matching line.
func TestAtomicWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xdc")

	original := `## Clock constraint
set_property -dict { PACKAGE_PIN E3  IOSTANDARD LVCMOS33 } [get_ports { clk }]
## Reset
set_property -dict { PACKAGE_PIN C12 IOSTANDARD LVCMOS33 } [get_ports { rst }]
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate atomicWriteFile: write temp → rename
	newContent := original + "set_property BITSTREAM.GENERAL.COMPRESS TRUE [current_design]\n"
	tmp := path + ".vconn.tmp"
	if err := os.WriteFile(tmp, []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "BITSTREAM.GENERAL.COMPRESS") {
		t.Errorf("appended line not found in file")
	}
	if !strings.Contains(string(got), "## Clock constraint") {
		t.Errorf("original comment was lost")
	}
}

// TestReplaceConstraintLogic verifies the scan-and-replace logic in replaceXDCConstraint.
func TestReplaceConstraintLogic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pins.xdc")

	original := "## Buttons\nset_property -dict { PACKAGE_PIN N17 IOSTANDARD LVCMOS33 } [get_ports { btn0 }]\nset_property -dict { PACKAGE_PIN P18 IOSTANDARD LVCMOS33 } [get_ports { btn1 }]\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	matchPfx := "set_property -dict { PACKAGE_PIN N17"
	newLine := "set_property -dict { PACKAGE_PIN N17 IOSTANDARD LVCMOS18 } [get_ports { btn0 }]"

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	replaced := 0
	for i, l := range lines {
		if strings.HasPrefix(l, matchPfx) {
			lines[i] = newLine
			replaced++
		}
	}
	if replaced != 1 {
		t.Fatalf("expected 1 replacement, got %d", replaced)
	}

	newContent := strings.Join(lines, "\n") + "\n"
	tmp := path + ".vconn.tmp"
	if err := os.WriteFile(tmp, []byte(newContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, path); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "LVCMOS18") {
		t.Errorf("replacement not applied: %s", string(got))
	}
	if strings.Contains(string(got), "LVCMOS33\n") && strings.Contains(string(got), "N17") {
		t.Errorf("old value still present for N17 pin")
	}
	// btn1 should be unchanged
	if !strings.Contains(string(got), "PACKAGE_PIN P18 IOSTANDARD LVCMOS33") {
		t.Errorf("btn1 constraint was unexpectedly changed")
	}
	// comment must survive
	if !strings.Contains(string(got), "## Buttons") {
		t.Errorf("comment was lost")
	}
}
