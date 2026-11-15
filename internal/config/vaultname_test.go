package config

import (
	"reflect"
	"testing"
)

func TestValidateVaultName_valid(t *testing.T) {
	for _, name := range []string{"myfish", "commons", "my-app", "myfish::restricted", "myfish::default"} {
		// act
		err := ValidateVaultName(name)
		// assert
		if err != nil {
			t.Errorf("ValidateVaultName(%q) unexpected error: %v", name, err)
		}
	}
}

func TestValidateVaultName_invalid(t *testing.T) {
	for _, name := range []string{"a::b::c", "::b", "a::", "a:b", ":::", ""} {
		// act
		err := ValidateVaultName(name)
		// assert
		if err == nil {
			t.Errorf("ValidateVaultName(%q) expected error", name)
		}
	}
}

func TestCanonicalVaultName(t *testing.T) {
	cases := map[string]string{
		"x::default":    "x",
		"x":             "x",
		"x::restricted": "x::restricted",
	}
	for in, want := range cases {
		// act
		got := CanonicalVaultName(in)
		// assert
		if got != want {
			t.Errorf("CanonicalVaultName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestKeyEnvNames(t *testing.T) {
	cases := map[string][]string{
		"myfish":                    {"WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT"},
		"myfish::default":           {"WARD_KEY_MYFISH", "WARD_KEY_MYFISH_DEFAULT"},
		"myfish::restricted":        {"WARD_KEY_MYFISH_RESTRICTED"},
		"messenger-api":             {"WARD_KEY_MESSENGER_API", "WARD_KEY_MESSENGER_API_DEFAULT"},
		"messenger-api::restricted": {"WARD_KEY_MESSENGER_API_RESTRICTED"},
	}
	for in, want := range cases {
		// act
		got := KeyEnvNames(in)
		// assert
		if !reflect.DeepEqual(got, want) {
			t.Errorf("KeyEnvNames(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestKeyFileCandidates(t *testing.T) {
	cases := map[string][]string{
		"myfish":             {".ward/myfish.key", ".ward/myfish::default.key"},
		"myfish::default":    {".ward/myfish.key", ".ward/myfish::default.key"},
		"myfish::restricted": {".ward/myfish::restricted.key"},
	}
	for in, want := range cases {
		// act
		got := KeyFileCandidates(in)
		// assert
		if !reflect.DeepEqual(got, want) {
			t.Errorf("KeyFileCandidates(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestSource_CanonicalName(t *testing.T) {
	cases := map[string]string{
		"myfish::default":    "myfish",
		"myfish":             "myfish",
		"myfish::restricted": "myfish::restricted",
	}
	for in, want := range cases {
		// arrange
		s := Source{Name: in}
		// act
		got := s.CanonicalName()
		// assert
		if got != want {
			t.Errorf("Source{%q}.CanonicalName() = %q, want %q", in, got, want)
		}
	}
}

func TestKeyEnvName_namespace_separator(t *testing.T) {
	// act
	got := KeyEnvName("myfish::restricted")
	// assert
	if got != "WARD_KEY_MYFISH_RESTRICTED" {
		t.Errorf("got %q", got)
	}
}
