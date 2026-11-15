package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeValueFile(t *testing.T, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "v")
	if err := os.WriteFile(p, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestResolveSetValue_literal_and_escape(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		literal    bool
		wantValue  string
		wantSource string
	}{
		{"plain literal", "postgres://x", false, "postgres://x", ""},
		{"double at escape", "@@org/x", false, "@org/x", ""},
		{"double at escape only strips first", "@@@x", false, "@@x", ""},
		{"literal flag keeps at path", "@/tmp/v", true, "@/tmp/v", ""},
		{"literal flag keeps double at", "@@x", true, "@@x", ""},
		{"literal flag keeps dash", "-", true, "-", ""},
		{"plain literal with literal flag", "abc", true, "abc", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			stdin := strings.NewReader("must not be read")

			// act
			value, source, err := resolveSetValue(tc.raw, tc.literal, stdin, false)

			// assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value != tc.wantValue {
				t.Errorf("value: want %q, got %q", tc.wantValue, value)
			}
			if source != tc.wantSource {
				t.Errorf("source: want %q, got %q", tc.wantSource, source)
			}
			if stdin.Len() != len("must not be read") {
				t.Errorf("stdin must not be consumed")
			}
		})
	}
}

func TestResolveSetValue_literal_flag_does_not_read_existing_file(t *testing.T) {
	// arrange
	p := writeValueFile(t, []byte("from-file\n"))

	// act
	value, source, err := resolveSetValue("@"+p, true, strings.NewReader(""), false)

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "@"+p || source != "" {
		t.Errorf("expected literal %q with empty source, got %q / %q", "@"+p, value, source)
	}
}

func TestResolveSetValue_file(t *testing.T) {
	json3 := "{\n  \"a\": 1,\n  \"b\": 2\n}"
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"trailing lf removed", "projects/1/locations/global/x\n", "projects/1/locations/global/x"},
		{"trailing crlf removed", "abc\r\n", "abc"},
		{"no trailing newline", "abc", "abc"},
		{"only one trailing lf removed", "abc\n\n", "abc\n"},
		{"internal newlines preserved", json3 + "\n", json3},
		{"internal crlf preserved", "a\r\nb\r\n", "a\r\nb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			p := writeValueFile(t, []byte(tc.content))

			// act
			value, source, err := resolveSetValue("@"+p, false, strings.NewReader(""), false)

			// assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value != tc.want {
				t.Errorf("value: want %q, got %q", tc.want, value)
			}
			if source != p {
				t.Errorf("source: want %q, got %q", p, source)
			}
		})
	}
}

func TestResolveSetValue_relative_file_resolves_against_invocation_dir(t *testing.T) {
	// arrange
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "v"), []byte("rel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	prev := invocationDir
	t.Cleanup(func() { invocationDir = prev })
	invocationDir = dir

	// act
	value, source, err := resolveSetValue("@v", false, strings.NewReader(""), false)

	// assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "rel" {
		t.Errorf("value: want %q, got %q", "rel", value)
	}
	if source != filepath.Join(dir, "v") {
		t.Errorf("source: want %q, got %q", filepath.Join(dir, "v"), source)
	}
}

func TestResolveSetValue_missing_file_errors_with_path_and_literal_hint(t *testing.T) {
	// arrange
	missing := filepath.Join(t.TempDir(), "nao-existe")

	// act
	_, _, err := resolveSetValue("@"+missing, false, strings.NewReader(""), false)

	// assert
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("expected error to contain path %q, got: %v", missing, err)
	}
	if !strings.Contains(err.Error(), "--literal") {
		t.Errorf("expected error to mention --literal, got: %v", err)
	}
}

func TestResolveSetValue_empty_content_errors(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"only lf", "\n"},
		{"only crlf", "\r\n"},
		{"zero bytes", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			p := writeValueFile(t, []byte(tc.content))

			// act
			_, _, err := resolveSetValue("@"+p, false, strings.NewReader(""), false)

			// assert
			if err == nil {
				t.Fatal("expected error for empty value")
			}
		})
	}
}

func TestResolveSetValue_invalid_utf8_errors(t *testing.T) {
	// arrange
	p := writeValueFile(t, []byte{0xff, 0xfe})

	// act
	_, _, err := resolveSetValue("@"+p, false, strings.NewReader(""), false)

	// assert
	if err == nil {
		t.Fatal("expected error for non UTF-8 content")
	}
	if !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("expected error to mention UTF-8, got: %v", err)
	}
}

func TestResolveSetValue_size_limit(t *testing.T) {
	t.Run("exactly one MiB is accepted", func(t *testing.T) {
		// arrange
		p := writeValueFile(t, bytes.Repeat([]byte("a"), 1048576))

		// act
		value, _, err := resolveSetValue("@"+p, false, strings.NewReader(""), false)

		// assert
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(value) != 1048576 {
			t.Errorf("expected 1048576 bytes, got %d", len(value))
		}
	})

	t.Run("one byte above the limit is rejected", func(t *testing.T) {
		// arrange
		p := writeValueFile(t, bytes.Repeat([]byte("a"), 1048577))

		// act
		_, _, err := resolveSetValue("@"+p, false, strings.NewReader(""), false)

		// assert
		if err == nil {
			t.Fatal("expected error above the limit")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "limit") {
			t.Errorf("expected error to mention the limit, got: %v", err)
		}
	})

	t.Run("limit is measured before removing the trailing newline", func(t *testing.T) {
		// arrange
		content := append(bytes.Repeat([]byte("a"), 1048576), '\n')
		p := writeValueFile(t, content)

		// act
		_, _, err := resolveSetValue("@"+p, false, strings.NewReader(""), false)

		// assert
		if err == nil {
			t.Fatal("expected error: 1048577 bytes before trimming")
		}
	})

	t.Run("stdin above the limit is rejected", func(t *testing.T) {
		// arrange
		stdin := bytes.NewReader(bytes.Repeat([]byte("a"), 1048577))

		// act
		_, _, err := resolveSetValue("-", false, stdin, false)

		// assert
		if err == nil {
			t.Fatal("expected error above the limit")
		}
	})
}

func TestResolveSetValue_stdin(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"lf removed", "s3cr3t\n", "s3cr3t"},
		{"crlf removed", "s3cr3t\r\n", "s3cr3t"},
		{"no newline", "s3cr3t", "s3cr3t"},
		{"multiline preserved", "a\nb\nc\n", "a\nb\nc"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// arrange
			stdin := strings.NewReader(tc.input)

			// act
			value, source, err := resolveSetValue("-", false, stdin, false)

			// assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value != tc.want {
				t.Errorf("value: want %q, got %q", tc.want, value)
			}
			if source != "stdin" {
				t.Errorf("source: want %q, got %q", "stdin", source)
			}
		})
	}
}

func TestResolveSetValue_stdin_terminal_errors_suggesting_pipe(t *testing.T) {
	// arrange
	stdin := strings.NewReader("ignored\n")

	// act
	_, _, err := resolveSetValue("-", false, stdin, true)

	// assert
	if err == nil {
		t.Fatal("expected error when stdin is a terminal")
	}
	if !strings.Contains(err.Error(), "pipe") || !strings.Contains(err.Error(), "terminal") {
		t.Errorf("expected error to mention pipe and terminal, got: %v", err)
	}
}

func TestResolveSetValue_stdin_empty_errors(t *testing.T) {
	cases := []string{"", "\n"}
	for _, input := range cases {
		// arrange
		stdin := strings.NewReader(input)

		// act
		_, _, err := resolveSetValue("-", false, stdin, false)

		// assert
		if err == nil {
			t.Errorf("expected error for stdin %q", input)
		}
	}
}

func TestResolveSetValue_stdin_invalid_utf8_errors(t *testing.T) {
	// arrange
	stdin := bytes.NewReader([]byte{0xff, 0xfe})

	// act
	_, _, err := resolveSetValue("-", false, stdin, false)

	// assert
	if err == nil || !strings.Contains(err.Error(), "UTF-8") {
		t.Errorf("expected UTF-8 error, got: %v", err)
	}
}
