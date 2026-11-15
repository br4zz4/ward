package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const maxSetValueBytes = 1 << 20

// resolveSetValue interprets the value argument of `ward set`:
// "@<path>" reads a file, "-" reads stdin, "@@x" stores "@x", and literal
// stores the argument untouched. source is non-empty when the value was read.
func resolveSetValue(raw string, literal bool, stdin io.Reader, stdinIsTTY bool) (value, source string, err error) {
	if literal {
		return raw, "", nil
	}
	switch {
	case strings.HasPrefix(raw, "@@"):
		return raw[1:], "", nil
	case strings.HasPrefix(raw, "@"):
		path := resolveInvocationPath(raw[1:])
		f, err := os.Open(path)
		if err != nil {
			return "", "", readValueError(path, raw, err)
		}
		defer f.Close()
		value, err := readSetValue(f)
		if err != nil {
			return "", "", readValueError(path, raw, err)
		}
		return value, path, nil
	case raw == "-":
		if stdinIsTTY {
			return "", "", errors.New(`"-" reads from stdin, but stdin is a terminal — pipe a value (e.g. pbpaste | ward set <scope> -)`)
		}
		value, err := readSetValue(stdin)
		if err != nil {
			return "", "", fmt.Errorf("reading value from stdin: %w", err)
		}
		return value, "stdin", nil
	}
	return raw, "", nil
}

func readValueError(path, raw string, err error) error {
	return fmt.Errorf("reading value from %s: %w (use --literal to store %q as-is)", path, err, raw)
}

func readSetValue(r io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxSetValueBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxSetValueBytes {
		return "", fmt.Errorf("value exceeds the %d byte limit", maxSetValueBytes)
	}
	if bytes.HasSuffix(data, []byte("\n")) {
		data = bytes.TrimSuffix(data, []byte("\n"))
		data = bytes.TrimSuffix(data, []byte("\r"))
	}
	if len(data) == 0 {
		return "", errors.New("value is empty")
	}
	if !utf8.Valid(data) {
		return "", errors.New("value is not valid UTF-8; base64-encode binary data first")
	}
	return string(data), nil
}
