package mcp

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestExecArgs_withScope(t *testing.T) {
	got := execArgs("", []string{"group.key1"}, false, "rails server")
	want := []string{"exec", "group.key1", "--", "rails", "server"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestExecArgs_multiScope_prefixed(t *testing.T) {
	got := execArgs("", []string{"vault1:group.key1", "vault2:group.key1"}, true, "env")
	want := []string{"exec", "--prefixed", "vault1:group.key1", "vault2:group.key1", "--", "env"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestExecArgs_noScope(t *testing.T) {
	got := execArgs("", nil, false, "env")
	want := []string{"exec", "--", "env"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestAiGetInstruction_withPath(t *testing.T) {
	// act
	got := aiGetInstruction("shared.db.password")

	// assert: valid JSON with path, sensitivity flag and shell command
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("expected valid JSON, got: %q (%v)", got, err)
	}
	if m["path"] != "shared.db.password" {
		t.Errorf("expected path in payload, got: %q", m["path"])
	}
	if m["sensitive"] != "true" {
		t.Errorf("expected sensitive=true, got: %q", m["sensitive"])
	}
	if !reflect.DeepEqual(strings.Fields(m["instructions"]), []string{"Run", "in", "a", "shell:", "ward", "get", "shared.db.password"}) {
		t.Errorf("unexpected instructions: %q", m["instructions"])
	}
}

func TestAiGetInstruction_emptyPath(t *testing.T) {
	// act
	got := aiGetInstruction("")

	// assert: instructions fall back to plain 'ward get'
	if !strings.Contains(got, "ward get\"") && !strings.Contains(got, "ward get ") {
		t.Errorf("expected instructions for full tree access, got: %q", got)
	}
}

func TestAiFileInstruction(t *testing.T) {
	// act
	got := aiFileInstruction("service-account.json")

	// assert: valid JSON with filename and shell command
	var m map[string]string
	if err := json.Unmarshal([]byte(got), &m); err != nil {
		t.Fatalf("expected valid JSON, got: %q (%v)", got, err)
	}
	if m["filename"] != "service-account.json" {
		t.Errorf("expected filename in payload, got: %q", m["filename"])
	}
	if m["sensitive"] != "true" {
		t.Errorf("expected sensitive=true, got: %q", m["sensitive"])
	}
	if !strings.Contains(m["instructions"], "ward file extract service-account.json <dest-dir>") {
		t.Errorf("unexpected instructions: %q", m["instructions"])
	}
}
