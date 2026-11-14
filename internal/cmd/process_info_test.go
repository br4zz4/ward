package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessInfo_self(t *testing.T) {
	// act
	cmdline, ppid := processInfo(os.Getpid())

	// assert
	if want := os.Getppid(); ppid != want {
		t.Errorf("processInfo(self) ppid = %d, want %d", ppid, want)
	}
	if cmdline == "" {
		t.Fatal("expected a non-empty command line for the test process")
	}
	if want := filepath.Base(os.Args[0]); !strings.Contains(cmdline, want) {
		t.Errorf("expected cmdline to mention %q, got %q", want, cmdline)
	}
}

func TestProcessInfo_missing_pid_returns_empty(t *testing.T) {
	// act
	cmdline, ppid := processInfo(999999)

	// assert
	if cmdline != "" {
		t.Errorf("expected empty cmdline for a missing pid, got %q", cmdline)
	}
	if ppid != 0 {
		t.Errorf("expected ppid 0 for a missing pid, got %d", ppid)
	}
}

func TestProcessInfo_pid_one_is_well_formed(t *testing.T) {
	// act
	cmdline, ppid := processInfo(1)

	// assert
	if cmdline != "" && ppid < 0 {
		t.Errorf("processInfo(1) = (%q, %d): negative ppid", cmdline, ppid)
	}
}
