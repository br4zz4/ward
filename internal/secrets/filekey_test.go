package secrets_test

import (
	"testing"

	"github.com/br4zz4/ward/internal/secrets"
)

func TestFileKey_json(t *testing.T) {
	// arrange
	filename := "service-account.json"

	// act
	got := secrets.FileKey(filename)

	// assert
	if got != "service_account_json" {
		t.Errorf("expected service_account_json, got %q", got)
	}
}

func TestFileKey_xml(t *testing.T) {
	// arrange
	filename := "credentials.xml"

	// act
	got := secrets.FileKey(filename)

	// assert
	if got != "credentials_xml" {
		t.Errorf("expected credentials_xml, got %q", got)
	}
}

func TestFileKey_yaml(t *testing.T) {
	// arrange
	filename := "config.yaml"

	// act
	got := secrets.FileKey(filename)

	// assert
	if got != "config_yaml" {
		t.Errorf("expected config_yaml, got %q", got)
	}
}

func TestFileKey_hyphens(t *testing.T) {
	// arrange
	filename := "my-service-account.json"

	// act
	got := secrets.FileKey(filename)

	// assert
	if got != "my_service_account_json" {
		t.Errorf("expected my_service_account_json, got %q", got)
	}
}

func TestFileKey_no_extension(t *testing.T) {
	// arrange
	filename := "credentials"

	// act
	got := secrets.FileKey(filename)

	// assert
	if got != "credentials" {
		t.Errorf("expected credentials, got %q", got)
	}
}

func TestFileKey_strips_path(t *testing.T) {
	// arrange
	filename := "/tmp/some/path/service-account.json"

	// act
	got := secrets.FileKey(filename)

	// assert
	if got != "service_account_json" {
		t.Errorf("expected service_account_json, got %q", got)
	}
}

func TestWardFilename_json(t *testing.T) {
	// arrange
	filename := "service-account.json"

	// act
	got := secrets.WardFilename(filename)

	// assert
	if got != "service-account.json.ward" {
		t.Errorf("expected service-account.json.ward, got %q", got)
	}
}

func TestWardFilename_xml(t *testing.T) {
	// arrange
	filename := "credentials.xml"

	// act
	got := secrets.WardFilename(filename)

	// assert
	if got != "credentials.xml.ward" {
		t.Errorf("expected credentials.xml.ward, got %q", got)
	}
}

func TestWardFilename_strips_path(t *testing.T) {
	// arrange
	filename := "/tmp/path/service-account.json"

	// act
	got := secrets.WardFilename(filename)

	// assert
	if got != "service-account.json.ward" {
		t.Errorf("expected service-account.json.ward, got %q", got)
	}
}

func TestOriginalFilename_json(t *testing.T) {
	// arrange
	wardFile := "service-account.json.ward"

	// act
	got, ok := secrets.OriginalFilename(wardFile)

	// assert
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "service-account.json" {
		t.Errorf("expected service-account.json, got %q", got)
	}
}

func TestOriginalFilename_xml(t *testing.T) {
	// arrange
	wardFile := "credentials.xml.ward"

	// act
	got, ok := secrets.OriginalFilename(wardFile)

	// assert
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "credentials.xml" {
		t.Errorf("expected credentials.xml, got %q", got)
	}
}

func TestOriginalFilename_not_a_file_secret(t *testing.T) {
	// arrange
	wardFile := "main.ward"

	// act
	_, ok := secrets.OriginalFilename(wardFile)

	// assert
	if ok {
		t.Error("expected ok=false for plain .ward file")
	}
}

func TestOriginalFilename_strips_path(t *testing.T) {
	// arrange
	wardFile := "/vaults/app/service-account.json.ward"

	// act
	got, ok := secrets.OriginalFilename(wardFile)

	// assert
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "service-account.json" {
		t.Errorf("expected service-account.json, got %q", got)
	}
}

func TestIsPlainFile(t *testing.T) {
	cases := map[string]bool{
		"config.plain.ward":       true,
		"a/b/config.plain.ward":   true,
		"main.ward":               false,
		"sa.json.ward":            false,
		"notplain.ward":           false,
		"plain.ward":              false, // no name before .plain
	}
	for in, want := range cases {
		if got := secrets.IsPlainFile(in); got != want {
			t.Errorf("IsPlainFile(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestStripWardSuffix(t *testing.T) {
	cases := map[string]string{
		"config.plain.ward":     "config",
		"a/b/config.plain.ward": "config",
		"main.ward":             "main",
		"sa.json.ward":          "sa.json",
	}
	for in, want := range cases {
		if got := secrets.StripWardSuffix(in); got != want {
			t.Errorf("StripWardSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOriginalFilename_plain_is_not_file_secret(t *testing.T) {
	if _, ok := secrets.OriginalFilename("config.plain.ward"); ok {
		t.Error("expected .plain.ward to not be a file-secret")
	}
	if _, ok := secrets.OriginalFilename("sa.json.ward"); !ok {
		t.Error("expected sa.json.ward to still be a file-secret")
	}
}
