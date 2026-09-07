package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func TestNewVersionCmd_prints_version(t *testing.T) {
	c := NewVersionCmd("0.2.9-test")
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs(nil)
	if err := c.Execute(); err != nil {
		t.Fatalf("Execute() unexpected error: %v", err)
	}
	if got := out.String(); got != "ward version 0.2.9-test\n" {
		t.Errorf("version output = %q, want %q", got, "ward version 0.2.9-test\n")
	}
}

func TestNewVersionCmd_use_is_version(t *testing.T) {
	c := NewVersionCmd("dev")
	if got := c.Use; got != "version" {
		t.Errorf("Use = %q, want %q", got, "version")
	}
}

var _ = cobra.Command{}