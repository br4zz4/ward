//go:build e2e

package check_test

import (
	"path/filepath"
	"testing"

	"github.com/br4zz4/ward/test/e2e/testutil"
)

func TestCheck_missing_dir_exits_2(t *testing.T) {
	// arrange
	missing := filepath.Join(t.TempDir(), "nope")

	// act
	out, stderr, code := testutil.Run(t, bin, t.TempDir(), "check", "--dir", missing, "app:main.token", "x")

	// assert
	if code != 2 {
		t.Errorf("expected exit 2, got %d (stderr %q)", code, stderr)
	}
	if out != "" {
		t.Errorf("stdout must be empty, got %q", out)
	}
}

func TestCheck_unknown_flag_exits_2(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "check", "--nope", "a", "b")

	// assert
	if code != 2 {
		t.Errorf("expected exit 2, got %d (stderr %q)", code, stderr)
	}
	if out != "" {
		t.Errorf("stdout must be empty, got %q", out)
	}
}

func TestCheck_unknown_flag_before_subcommand_exits_2(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	out, stderr, code := testutil.Run(t, bin, dir, "--nope", "check", "a", "b")

	// assert
	if code != 2 {
		t.Errorf("expected exit 2, got %d (stderr %q)", code, stderr)
	}
	if out != "" {
		t.Errorf("stdout must be empty, got %q", out)
	}
}

func TestCheck_unknown_flag_before_subcommand_without_args_exits_2(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "--nope", "check")

	// assert
	if code != 2 {
		t.Errorf("expected exit 2, got %d (stderr %q)", code, stderr)
	}
}

func TestGet_unknown_flag_before_subcommand_still_exits_1(t *testing.T) {
	// arrange
	dir := project(t)

	// act
	_, stderr, code := testutil.Run(t, bin, dir, "--nope", "get", "a")

	// assert
	if code != 1 {
		t.Errorf("expected exit 1, got %d (stderr %q)", code, stderr)
	}
}
