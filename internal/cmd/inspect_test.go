package cmd

import (
	"strings"
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

// ── formatInspectSummary ──────────────────────────────────────────────────────

func TestFormatInspectSummary_all_zero(t *testing.T) {
	// arrange
	r := inspectResult{}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "0 conflicts") {
		t.Errorf("expected '0 conflicts' in summary, got: %q", clean)
	}
	if !strings.Contains(clean, "0 env collisions") {
		t.Errorf("expected '0 env collisions' in summary, got: %q", clean)
	}
	if !strings.Contains(clean, "0 structure errors") {
		t.Errorf("expected '0 structure errors' in summary, got: %q", clean)
	}
}

func TestFormatInspectSummary_with_conflicts(t *testing.T) {
	// arrange
	r := inspectResult{
		conflictErr: &secrets.ConflictError{
			Conflicts: []secrets.Conflict{
				{Key: "app.db.url"},
				{Key: "app.db.pass"},
			},
		},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "2 conflicts") {
		t.Errorf("expected '2 conflicts' in summary, got: %q", clean)
	}
}

func TestFormatInspectSummary_singular_conflict(t *testing.T) {
	// arrange
	r := inspectResult{
		conflictErr: &secrets.ConflictError{
			Conflicts: []secrets.Conflict{{Key: "app.key"}},
		},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "1 conflict") || strings.Contains(clean, "1 conflicts") {
		t.Errorf("expected singular '1 conflict' in summary, got: %q", clean)
	}
}

func TestFormatInspectSummary_with_env_collisions(t *testing.T) {
	// arrange
	r := inspectResult{
		envConflictErr: &secrets.EnvConflictError{
			Conflicts: []secrets.EnvConflict{
				{EnvKey: "TOKEN"},
				{EnvKey: "SECRET"},
				{EnvKey: "KEY"},
			},
		},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "3 env collisions") {
		t.Errorf("expected '3 env collisions' in summary, got: %q", clean)
	}
}

func TestFormatInspectSummary_singular_env_collision(t *testing.T) {
	// arrange
	r := inspectResult{
		envConflictErr: &secrets.EnvConflictError{
			Conflicts: []secrets.EnvConflict{{EnvKey: "TOKEN"}},
		},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "1 env collision") || strings.Contains(clean, "1 env collisions") {
		t.Errorf("expected singular '1 env collision' in summary, got: %q", clean)
	}
}

func TestFormatInspectSummary_with_structure_errors(t *testing.T) {
	// arrange
	r := inspectResult{
		structureViolations: []string{
			`file "a.ward": key path "wrong" does not match expected "app.a"`,
			`file "b.ward": key path "bad" does not match expected "app.b"`,
			`file "c.ward": key path "x" does not match expected "app.c"`,
			`file "d.ward": key path "y" does not match expected "app.d"`,
		},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "4 structure errors") {
		t.Errorf("expected '4 structure errors' in summary, got: %q", clean)
	}
}

func TestFormatInspectSummary_singular_structure_error(t *testing.T) {
	// arrange
	r := inspectResult{
		structureViolations: []string{`file "a.ward": key path "wrong" does not match expected "app.a"`},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "1 structure error") || strings.Contains(clean, "1 structure errors") {
		t.Errorf("expected singular '1 structure error' in summary, got: %q", clean)
	}
}

func TestFormatInspectSummary_all_errors_combined(t *testing.T) {
	// arrange
	r := inspectResult{
		conflictErr: &secrets.ConflictError{
			Conflicts: []secrets.Conflict{{Key: "app.key"}},
		},
		envConflictErr: &secrets.EnvConflictError{
			Conflicts: []secrets.EnvConflict{{EnvKey: "TOKEN"}},
		},
		structureViolations: []string{
			`file "a.ward": key path "x" does not match expected "app.a"`,
			`file "b.ward": key path "y" does not match expected "app.b"`,
			`file "c.ward": key path "z" does not match expected "app.c"`,
			`file "d.ward": key path "w" does not match expected "app.d"`,
		},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	clean := stripANSICmd(out)
	if !strings.Contains(clean, "1 conflict") {
		t.Errorf("expected '1 conflict', got: %q", clean)
	}
	if !strings.Contains(clean, "1 env collision") {
		t.Errorf("expected '1 env collision', got: %q", clean)
	}
	if !strings.Contains(clean, "4 structure errors") {
		t.Errorf("expected '4 structure errors', got: %q", clean)
	}
}

func TestFormatInspectSummary_clean_uses_checkmark(t *testing.T) {
	// arrange
	r := inspectResult{}

	// act
	out := formatInspectSummary(r)

	// assert
	if !strings.Contains(out, "✓") {
		t.Errorf("expected checkmark in clean summary, got: %q", out)
	}
}

func TestFormatInspectSummary_errors_use_cross(t *testing.T) {
	// arrange
	r := inspectResult{
		structureViolations: []string{"some violation"},
	}

	// act
	out := formatInspectSummary(r)

	// assert
	if !strings.Contains(out, "✗") {
		t.Errorf("expected cross mark in error summary, got: %q", out)
	}
}

// ── inspectResult.hasErrors ───────────────────────────────────────────────────

func TestInspectResult_hasErrors_false_when_empty(t *testing.T) {
	// arrange
	r := inspectResult{}

	// act + assert
	if r.hasErrors() {
		t.Error("expected hasErrors to be false for empty result")
	}
}

func TestInspectResult_hasErrors_true_with_conflict(t *testing.T) {
	// arrange
	r := inspectResult{
		conflictErr: &secrets.ConflictError{Conflicts: []secrets.Conflict{{Key: "x"}}},
	}

	// act + assert
	if !r.hasErrors() {
		t.Error("expected hasErrors to be true when conflictErr is set")
	}
}

func TestInspectResult_hasErrors_true_with_env_collision(t *testing.T) {
	// arrange
	r := inspectResult{
		envConflictErr: &secrets.EnvConflictError{Conflicts: []secrets.EnvConflict{{EnvKey: "X"}}},
	}

	// act + assert
	if !r.hasErrors() {
		t.Error("expected hasErrors to be true when envConflictErr is set")
	}
}

func TestInspectResult_hasErrors_true_with_structure_violations(t *testing.T) {
	// arrange
	r := inspectResult{
		structureViolations: []string{"violation"},
	}

	// act + assert
	if !r.hasErrors() {
		t.Error("expected hasErrors to be true when structureViolations is non-empty")
	}
}
