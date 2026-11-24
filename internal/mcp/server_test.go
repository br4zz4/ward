package mcp

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const fakeWardArgsEnv = "WARD_MCP_TEST_ARGS_FILE"
const fakeWardRefuseEnv = "WARD_MCP_TEST_REFUSE"
const fakeWardAIModeFileEnv = "WARD_MCP_TEST_AIMODE_FILE"
const fakeWardExitEnv = "WARD_MCP_TEST_EXIT"
const fakeWardStdoutEnv = "WARD_MCP_TEST_STDOUT"
const fakeWardStderrEnv = "WARD_MCP_TEST_STDERR"

func TestMain(m *testing.M) {
	if out := os.Getenv(fakeWardAIModeFileEnv); out != "" {
		os.WriteFile(out, []byte(os.Getenv("WARD_AI_MODE")), 0o600)
	}
	if exit := os.Getenv(fakeWardExitEnv); exit != "" {
		if out := os.Getenv(fakeWardArgsEnv); out != "" {
			b, _ := json.Marshal(os.Args[1:])
			os.WriteFile(out, b, 0o600)
		}
		os.Stdout.WriteString(os.Getenv(fakeWardStdoutEnv))
		os.Stderr.WriteString(os.Getenv(fakeWardStderrEnv))
		code, _ := strconv.Atoi(exit)
		os.Exit(code)
	}
	if os.Getenv(fakeWardRefuseEnv) != "" {
		os.Stderr.WriteString("ward: refused in AI mode: decrypted content never enters the agent context\n")
		os.Exit(3)
	}
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
	got := aiGetInstruction("shared.db.password", "")

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
	got := aiGetInstruction("", "")

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

func TestWardRaw_runs_in_ai_mode_and_returns_refusal_never_yaml(t *testing.T) {
	// arrange
	aiFile := filepath.Join(t.TempDir(), "aimode.txt")
	t.Setenv(fakeWardAIModeFileEnv, aiFile)
	t.Setenv(fakeWardRefuseEnv, "1")
	call := `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"ward_raw","arguments":{"file":".ward/vaults/app/main.plain.ward"}}}`

	// act
	got := driveServer(t, []string{initRequest, initializedNotification, call}, []string{"call"})

	// assert
	b, err := os.ReadFile(aiFile)
	if err != nil {
		t.Fatalf("ward subprocess was not invoked: %v", err)
	}
	if string(b) != "1" {
		t.Errorf("subprocess must run with WARD_AI_MODE=1, got %q", string(b))
	}
	var res struct {
		IsError bool `json:"isError"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(got["call"], &res); err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Errorf("ward_raw must return a tool error when the CLI refuses, got: %s", got["call"])
	}
	if len(res.Content) == 0 || !strings.Contains(res.Content[0].Text, "AI mode") {
		t.Errorf("expected refusal text, got: %s", got["call"])
	}
}

func TestWardRaw_and_export_descriptions_say_refused_in_ai_mode(t *testing.T) {
	// arrange
	list := `{"jsonrpc":"2.0","id":"list","method":"tools/list"}`

	// act
	got := driveServer(t, []string{initRequest, initializedNotification, list}, []string{"list"})

	// assert
	var res struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(got["list"], &res); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, tool := range res.Tools {
		if tool.Name != "ward_raw" && tool.Name != "ward_export" {
			continue
		}
		seen[tool.Name] = true
		if !strings.Contains(strings.ToLower(tool.Description), "refused") || !strings.Contains(strings.ToLower(tool.Description), "ai mode") {
			t.Errorf("%s description must say it is refused in AI mode, got: %q", tool.Name, tool.Description)
		}
	}
	if !seen["ward_raw"] {
		t.Fatal("ward_raw tool not listed")
	}
}

type toolResult struct {
	IsError bool `json:"isError"`
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

func callWardGet(t *testing.T, args string) toolResult {
	t.Helper()
	call := `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"ward_get","arguments":` + args + `}}`
	got := driveServer(t, []string{initRequest, initializedNotification, call}, []string{"call"})
	var res toolResult
	if err := json.Unmarshal(got["call"], &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestWardGet_masked_level_returns_masked_field(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)
	t.Setenv(fakeWardExitEnv, "0")
	t.Setenv(fakeWardStdoutEnv, "tok_…a9f2 (51 chars)\n")

	// act
	res := callWardGet(t, `{"path":"app:main.token","dir":"/proj"}`)

	// assert
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("expected a successful result, got %+v", res)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(res.Content[0].Text), &m); err != nil {
		t.Fatalf("expected JSON payload, got %q (%v)", res.Content[0].Text, err)
	}
	if m["masked"] != "tok_…a9f2 (51 chars)" {
		t.Errorf("masked = %q", m["masked"])
	}
	if m["path"] != "app:main.token" || m["sensitive"] != "true" {
		t.Errorf("unexpected payload: %v", m)
	}
	if !strings.Contains(m["instructions"], "ward exec") {
		t.Errorf("instructions must keep the exec guidance: %q", m["instructions"])
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ward subprocess was not invoked: %v", err)
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	want := []string{"-d", "/proj", "get", "app:main.token"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("got args %v want %v", args, want)
	}
}

func TestWardGet_strict_level_has_no_masked_field(t *testing.T) {
	// arrange
	t.Setenv(fakeWardExitEnv, "3")
	t.Setenv(fakeWardStderrEnv, "ward: refused in AI mode\n")

	// act
	res := callWardGet(t, `{"path":"app:main.token"}`)

	// assert
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("a refusal must still return the instruction payload, got %+v", res)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &m); err != nil {
		t.Fatalf("expected JSON payload, got %q (%v)", res.Content[0].Text, err)
	}
	if _, has := m["masked"]; has {
		t.Errorf("strict payload must not carry a masked field: %v", m)
	}
	if m["path"] != "app:main.token" {
		t.Errorf("unexpected payload: %v", m)
	}
}

func TestWardGet_strict_refusal_instructions_guide_three_exits(t *testing.T) {
	// arrange
	t.Setenv(fakeWardExitEnv, "3")
	t.Setenv(fakeWardStderrEnv, "ward: refused in AI mode\n")

	// act
	res := callWardGet(t, `{"path":"app:main.token"}`)

	// assert
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("expected the instruction payload, got %+v", res)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].Text), &m); err != nil {
		t.Fatalf("expected JSON payload, got %q (%v)", res.Content[0].Text, err)
	}
	instructions, _ := m["instructions"].(string)
	for _, want := range []string{"ward exec", "ward check", "ward set", "--from"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("instructions must mention %q, got: %q", want, instructions)
		}
	}
	for field, v := range m {
		if s, ok := v.(string); ok && strings.Contains(s, "ward get") {
			t.Errorf("field %q must not suggest running ward get: %q", field, s)
		}
	}
}

func TestWardDocs_ai_mode_section_lists_three_exits_without_ward_get(t *testing.T) {
	// arrange
	call := `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"ward_docs","arguments":{}}}`

	// act
	got := driveServer(t, []string{initRequest, initializedNotification, call}, []string{"call"})

	// assert
	var res toolResult
	if err := json.Unmarshal(got["call"], &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("empty docs: %s", got["call"])
	}
	docs := res.Content[0].Text
	idx := strings.Index(docs, "## AI mode")
	if idx < 0 {
		t.Fatalf("docs must have a '## AI mode' section")
	}
	section := docs[idx+len("## AI mode"):]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}
	for _, want := range []string{"ward exec", "ward check", "ward set", "--from"} {
		if !strings.Contains(section, want) {
			t.Errorf("AI mode docs must mention %q", want)
		}
	}
	if strings.Contains(docs, "ward get vault1:group.key1") {
		t.Errorf("docs must not instruct running `ward get vault1:group.key1`")
	}
	for _, banned := range []string{"ward get vault1:group.key1", "run: ward get", "run ward get"} {
		if strings.Contains(section, banned) {
			t.Errorf("AI mode section must not instruct running ward get, found %q", banned)
		}
	}
}

func TestWardGet_unknown_path_is_tool_error(t *testing.T) {
	// arrange
	t.Setenv(fakeWardExitEnv, "1")
	t.Setenv(fakeWardStderrEnv, "ward: secret not found: app:nope\n")

	// act
	res := callWardGet(t, `{"path":"app:nope"}`)

	// assert
	if !res.IsError {
		t.Errorf("expected a tool error for an unknown path, got %+v", res)
	}
	if len(res.Content) > 0 && strings.Contains(res.Content[0].Text, `"masked"`) {
		t.Errorf("error must not carry a masked field: %q", res.Content[0].Text)
	}
}

func callWardCheck(t *testing.T, args string) toolResult {
	t.Helper()
	call := `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"ward_check","arguments":` + args + `}}`
	got := driveServer(t, []string{initRequest, initializedNotification, call}, []string{"call"})
	var res toolResult
	if err := json.Unmarshal(got["call"], &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestWardCheck_match_invokes_cli_with_value_file(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)
	t.Setenv(fakeWardExitEnv, "0")
	t.Setenv(fakeWardStdoutEnv, "match\n")

	// act
	res := callWardCheck(t, `{"path":"app:k","value_file":"/tmp/v","dir":"/proj"}`)

	// assert
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("expected a successful result, got %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "match") || strings.Contains(res.Content[0].Text, "differs") {
		t.Errorf("expected match, got %q", res.Content[0].Text)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ward subprocess was not invoked: %v", err)
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	want := []string{"-d", "/proj", "check", "--", "app:k", "@/tmp/v"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("got args %v want %v", args, want)
	}
}

func TestWardCheck_without_dir_omits_dir_flag(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)
	t.Setenv(fakeWardExitEnv, "0")
	t.Setenv(fakeWardStdoutEnv, "match\n")

	// act
	res := callWardCheck(t, `{"path":"app:k","value_file":"/tmp/v"}`)

	// assert
	if res.IsError {
		t.Fatalf("expected success, got %+v", res)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ward subprocess was not invoked: %v", err)
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	want := []string{"check", "--", "app:k", "@/tmp/v"}
	if !reflect.DeepEqual(args, want) {
		t.Errorf("got args %v want %v", args, want)
	}
}

func TestWardCheck_differs_is_success_not_tool_error(t *testing.T) {
	// arrange
	t.Setenv(fakeWardExitEnv, "1")
	t.Setenv(fakeWardStdoutEnv, "differs\n")

	// act
	res := callWardCheck(t, `{"path":"app:k","value_file":"/tmp/v"}`)

	// assert
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("differs must be a successful result, got %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "differs") {
		t.Errorf("expected differs, got %q", res.Content[0].Text)
	}
}

func TestWardCheck_exit_2_is_tool_error_with_stderr(t *testing.T) {
	// arrange
	t.Setenv(fakeWardExitEnv, "2")
	t.Setenv(fakeWardStderrEnv, "ward: secret not found: app:nope\n")

	// act
	res := callWardCheck(t, `{"path":"app:nope","value_file":"/tmp/v"}`)

	// assert
	if !res.IsError || len(res.Content) == 0 {
		t.Fatalf("expected a tool error, got %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "secret not found: app:nope") {
		t.Errorf("error must carry stderr, got %q", res.Content[0].Text)
	}
}

func TestWardCheck_other_exit_code_is_tool_error(t *testing.T) {
	// arrange
	t.Setenv(fakeWardExitEnv, "3")
	t.Setenv(fakeWardStderrEnv, "ward: boom\n")

	// act
	res := callWardCheck(t, `{"path":"app:k","value_file":"/tmp/v"}`)

	// assert
	if !res.IsError {
		t.Errorf("expected a tool error for exit 3, got %+v", res)
	}
}

func TestWardCheck_schema_has_no_literal_value(t *testing.T) {
	// arrange
	list := `{"jsonrpc":"2.0","id":"list","method":"tools/list"}`

	// act
	got := driveServer(t, []string{initRequest, initializedNotification, list}, []string{"list"})

	// assert
	var res struct {
		Tools []struct {
			Name        string `json:"name"`
			InputSchema struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
			} `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(got["list"], &res); err != nil {
		t.Fatal(err)
	}
	for _, tool := range res.Tools {
		if tool.Name != "ward_check" {
			continue
		}
		for _, p := range []string{"path", "value_file", "dir"} {
			if _, ok := tool.InputSchema.Properties[p]; !ok {
				t.Errorf("ward_check must declare %q", p)
			}
		}
		if _, has := tool.InputSchema.Properties["value"]; has {
			t.Errorf("ward_check must not accept a literal value")
		}
		if len(tool.InputSchema.Properties) != 3 {
			t.Errorf("expected exactly path, value_file and dir, got %v", tool.InputSchema.Properties)
		}
		return
	}
	t.Fatal("ward_check tool not listed")
}

func callWardSet(t *testing.T, args string) toolResult {
	t.Helper()
	call := `{"jsonrpc":"2.0","id":"call","method":"tools/call","params":{"name":"ward_set","arguments":` + args + `}}`
	got := driveServer(t, []string{initRequest, initializedNotification, call}, []string{"call"})
	var res toolResult
	if err := json.Unmarshal(got["call"], &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func readSubprocessArgs(t *testing.T, argsFile string) []string {
	t.Helper()
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("ward subprocess was not invoked: %v", err)
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	return args
}

func TestWardSet_from_invokes_cli_with_from_flag(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)

	// act
	res := callWardSet(t, `{"path":"other:main.k","from":"app:main.k"}`)

	// assert
	if res.IsError {
		t.Fatalf("expected success, got %+v", res)
	}
	want := []string{"set", "--from", "app:main.k", "--", "other:main.k"}
	if got := readSubprocessArgs(t, argsFile); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestWardSet_from_with_dir_prepends_dir_flag(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)

	// act
	res := callWardSet(t, `{"path":"other:main.k","from":"app:main.k","dir":"/proj"}`)

	// assert
	if res.IsError {
		t.Fatalf("expected success, got %+v", res)
	}
	want := []string{"-d", "/proj", "set", "--from", "app:main.k", "--", "other:main.k"}
	if got := readSubprocessArgs(t, argsFile); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestWardSet_value_and_from_together_is_tool_error_without_running_cli(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)

	// act
	res := callWardSet(t, `{"path":"other:main.k","value":"v","from":"app:main.k"}`)

	// assert
	if !res.IsError {
		t.Errorf("expected a tool error, got %+v", res)
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Errorf("ward subprocess must not run when value and from are both given")
	}
}

func TestWardSet_without_value_nor_from_is_tool_error_without_running_cli(t *testing.T) {
	// arrange
	argsFile := filepath.Join(t.TempDir(), "args.json")
	t.Setenv(fakeWardArgsEnv, argsFile)

	// act
	res := callWardSet(t, `{"path":"other:main.k"}`)

	// assert
	if !res.IsError {
		t.Errorf("expected a tool error, got %+v", res)
	}
	if _, err := os.Stat(argsFile); err == nil {
		t.Errorf("ward subprocess must not run when neither value nor from is given")
	}
}

func TestWardSet_schema_value_optional_and_from_declared(t *testing.T) {
	// arrange
	list := `{"jsonrpc":"2.0","id":"list","method":"tools/list"}`

	// act
	got := driveServer(t, []string{initRequest, initializedNotification, list}, []string{"list"})

	// assert
	var res struct {
		Tools []struct {
			Name        string `json:"name"`
			InputSchema struct {
				Properties map[string]json.RawMessage `json:"properties"`
				Required   []string                   `json:"required"`
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
		if _, ok := tool.InputSchema.Properties["from"]; !ok {
			t.Errorf("ward_set must declare from")
		}
		for _, r := range tool.InputSchema.Required {
			if r == "value" || r == "from" {
				t.Errorf("%q must not be required on its own, required: %v", r, tool.InputSchema.Required)
			}
		}
		return
	}
	t.Fatal("ward_set tool not listed")
}
