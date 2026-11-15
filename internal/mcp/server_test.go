package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

const fakeWardArgsEnv = "WARD_MCP_TEST_ARGS_FILE"

func TestMain(m *testing.M) {
	if out := os.Getenv(fakeWardArgsEnv); out != "" {
		b, _ := json.Marshal(os.Args[1:])
		os.WriteFile(out, b, 0o600)
		os.Stdout.WriteString("ok")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func driveServer(t *testing.T, requests []string, waitForIDs []string) map[string]json.RawMessage {
	t.Helper()
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	prevIn, prevOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW

	collected := &lockedBuffer{}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		sc := bufio.NewScanner(outR)
		sc.Buffer(make([]byte, 0, 1<<20), 8<<20)
		for sc.Scan() {
			collected.Write([]byte(sc.Text() + "\n"))
		}
	}()

	serveDone := make(chan error, 1)
	go func() { serveDone <- Serve() }()

	for _, r := range requests {
		inW.Write([]byte(r + "\n"))
	}

	responses := func() map[string]json.RawMessage {
		out := map[string]json.RawMessage{}
		for _, line := range strings.Split(collected.String(), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var msg struct {
				ID     json.RawMessage `json:"id"`
				Result json.RawMessage `json:"result"`
			}
			if json.Unmarshal([]byte(line), &msg) == nil && len(msg.ID) > 0 {
				out[strings.Trim(string(msg.ID), `"`)] = msg.Result
			}
		}
		return out
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		got := responses()
		all := true
		for _, id := range waitForIDs {
			if _, ok := got[id]; !ok {
				all = false
			}
		}
		if all {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	inW.Close()
	select {
	case <-serveDone:
	case <-time.After(10 * time.Second):
		t.Fatal("server did not stop after stdin closed")
	}
	os.Stdin, os.Stdout = prevIn, prevOut
	outW.Close()
	<-readDone
	inR.Close()
	outR.Close()

	got := responses()
	for _, id := range waitForIDs {
		if _, ok := got[id]; !ok {
			t.Fatalf("no response for request id %s; output: %q", id, collected.String())
		}
	}
	return got
}

const initRequest = `{"jsonrpc":"2.0","id":"init","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`
const initializedNotification = `{"jsonrpc":"2.0","method":"notifications/initialized"}`

func TestWardSet_invokes_cli_with_literal_flag_and_separator(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)
	call := `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"ward_set","arguments":{"path":"app:main.token","value":"@/etc/hosts"}}}`

	// act
	driveServer(t, []string{initRequest, initializedNotification, call}, []string{"call"})

	// assert
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ward subprocess was not invoked: %v", err)
	}
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"set", "--literal", "--", "app:main.token", "@/etc/hosts"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestWardSet_with_dir_prepends_dir_flag(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)
	call := `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"ward_set","arguments":{"path":"app:main.token","value":"-","dir":"/proj"}}}`

	// act
	driveServer(t, []string{initRequest, initializedNotification, call}, []string{"call"})

	// assert
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ward subprocess was not invoked: %v", err)
	}
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"-d", "/proj", "set", "--literal", "--", "app:main.token", "-"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestWardSet_value_description_says_literal(t *testing.T) {
	// arrange
	list := `{"jsonrpc":"2.0","id":"list","method":"tools/list"}`

	// act
	got := driveServer(t, []string{initRequest, initializedNotification, list}, []string{"list"})

	// assert
	var res struct {
		Tools []struct {
			Name        string `json:"name"`
			InputSchema struct {
				Properties map[string]struct {
					Description string `json:"description"`
				} `json:"properties"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(got["list"], &res); err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "ward_set" {
			continue
		}
		desc := strings.ToLower(tool.InputSchema.Properties["value"].Description)
		if !strings.Contains(desc, "as-is") && !strings.Contains(desc, "literal") {
			t.Errorf("value description must say it is stored literally, got: %q", desc)
		}
		if !strings.Contains(desc, "@") || !strings.Contains(desc, "-") {
			t.Errorf("value description must mention that '@' and '-' are not interpreted, got: %q", desc)
		}
		return
	}
	t.Fatal("ward_set tool not listed")
}

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

	// assert: valid JSON with path, sensitivity flag and guidance
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
	// The guidance must NOT tell the agent to run `ward get` (that would read
	// the value). It must say never to read it and how to EXECUTE using it.
	if strings.Contains(m["instructions"], "ward get ") || strings.Contains(m["instructions"], "ward get\"") {
		t.Errorf("instructions must not suggest running ward get: %q", m["instructions"])
	}
	if !strings.Contains(m["instructions"], "ward exec") {
		t.Errorf("instructions must show how to execute using the secret: %q", m["instructions"])
	}
	lower := strings.ToLower(m["instructions"])
	if !strings.Contains(lower, "never") {
		t.Errorf("instructions must say to never read the value: %q", m["instructions"])
	}
}

func TestAiGetInstruction_emptyPath(t *testing.T) {
	// act
	got := aiGetInstruction("")

	// assert: guidance explains execution via ward exec, never reading
	if strings.Contains(got, "ward get") {
		t.Errorf("instructions must not suggest running ward get: %q", got)
	}
	if !strings.Contains(got, "ward exec") {
		t.Errorf("expected instructions to show ward exec usage, got: %q", got)
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
