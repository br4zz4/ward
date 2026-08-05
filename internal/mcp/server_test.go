package mcp

import (
	"reflect"
	"testing"
)

func TestExecArgs_withScope(t *testing.T) {
	got := execArgs("", []string{"infra.staging"}, false, "rails server")
	want := []string{"exec", "infra.staging", "--", "rails", "server"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestExecArgs_multiScope_prefixed(t *testing.T) {
	got := execArgs("", []string{"commons:infra.staging", "trgclub:infra.staging"}, true, "env")
	want := []string{"exec", "--prefixed", "commons:infra.staging", "trgclub:infra.staging", "--", "env"}
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
