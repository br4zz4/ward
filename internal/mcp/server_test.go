package mcp

import (
	"reflect"
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
