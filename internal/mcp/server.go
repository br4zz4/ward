package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// dirArgs prepends -d <dir> to args when dir is non-empty.
func dirArgs(dir string, args ...string) []string {
	if dir == "" {
		return args
	}
	return append([]string{"-d", dir}, args...)
}

// execArgs assembles ward exec args: optional -d dir, --prefixed, zero-or-more
// scopes, then -- and the command tokens.
func execArgs(dir string, scopes []string, prefixed bool, command string) []string {
	args := []string{"exec"}
	if prefixed {
		args = append(args, "--prefixed")
	}
	args = append(args, scopes...)
	args = append(args, "--")
	args = append(args, strings.Fields(command)...)
	return dirArgs(dir, args...)
}

func run(args ...string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		bin = "ward"
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = aiModeEnv()
	out, err := cmd.CombinedOutput()
	text := stripANSI(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("%s", text)
	}
	return text, nil
}

const exitAIRefusal = 3

// runExit runs ward and returns its stdout and exit status. A non-zero status
// is not an error here: callers decide what each status means. err is set only
// when the process could not be run at all.
func runExit(args ...string) (stdout string, code int, err error) {
	bin, exeErr := os.Executable()
	if exeErr != nil {
		bin = "ward"
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = aiModeEnv()
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	text := stripANSI(strings.TrimSpace(outBuf.String()))
	if runErr == nil {
		return text, 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		if exitErr.ExitCode() != exitAIRefusal {
			text = stripANSI(strings.TrimSpace(errBuf.String()))
		}
		return text, exitErr.ExitCode(), nil
	}
	return "", -1, runErr
}

func runWithStdin(stdin string, args ...string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		bin = "ward"
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	cmd.Env = aiModeEnv()
	out, err := cmd.CombinedOutput()
	text := stripANSI(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("%s", text)
	}
	return text, nil
}

// aiModeEnv returns the child process environment with WARD_AI_MODE=1 set, so
// every ward subprocess spawned by the MCP server runs in AI mode — values are
// never exposed to the agent, and exec refuses env-dumping commands.
func aiModeEnv() []string {
	return append(os.Environ(), "WARD_AI_MODE=1")
}

func ok(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

func fail(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
}

// aiGetInstruction builds the structured JSON reply for ward_get in AI mode:
// the value is never exposed, and the guidance must NOT tell the agent to run
// `ward get` (that would read it). The agent may EXECUTE a command that uses
// the secret, but must never READ the value.
func aiGetInstruction(path, masked string) string {
	secretRef := "this secret"
	if path != "" {
		secretRef = "the secret at '" + path + "'"
	}
	instructions := "Never read " + secretRef + ". To use it in a command without "+
		"seeing it, run: ward exec -- sh -c '<command using the env var>'"
	payload := map[string]string{
		"path":         path,
		"sensitive":    "true",
		"description":  "No description available",
		"instructions": instructions,
		"hint":         "Ward never exposes secret values in AI context. You may EXECUTE commands that use the secret (ward exec), but you must never READ it.",
	}
	if masked != "" {
		payload["masked"] = masked
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("ward: AI mode: never read this secret; execute commands that use it via 'ward exec -- sh -c <cmd>'")
	}
	return string(out)
}

// aiFileInstruction builds the JSON reply for ward_file_extract in AI mode.
func aiFileInstruction(filename string) string {
	payload := map[string]string{
		"filename":     filename,
		"sensitive":    "true",
		"instructions": "Run in a shell: ward file extract " + filename + " <dest-dir>",
		"hint":         "Ward file secrets are never exposed in AI context. Use bash to write the file to disk without reading it.",
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("ward: AI mode: run 'ward file extract %s <dest-dir>' in a shell to write the file to disk", filename)
	}
	return string(out)
}

func Serve() error {
	s := server.NewMCPServer("ward", "1.0.0",
		server.WithToolCapabilities(true),
	)

	s.AddTool(
		mcp.NewTool("ward_docs",
			mcp.WithDescription("Get documentation about ward: concepts, CLI usage, and available Claude Code skills"),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return ok(wardDocs), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_get",
			mcp.WithDescription("Return the merged value at a dot-path (or full tree if no path given). "+
				"In AI mode the value is never returned: at the strict level the tool replies with how to use it "+
				"(ward exec); at the masked level it also returns 'masked', the first and last characters and the length."),
			mcp.WithString("path", mcp.Description("scope to a secret: 'vault1:group.key1' (one vault) or 'group.key1' (searches all vaults; ambiguous = error)")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			p := req.GetString("path", "")
			if p == "" {
				return ok(aiGetInstruction("", "")), nil
			}
			out, code, err := runExit(dirArgs(req.GetString("dir", ""), "get", p)...)
			if err != nil {
				return fail(err), nil
			}
			switch code {
			case 0:
				return ok(aiGetInstruction(p, out)), nil
			case exitAIRefusal:
				return ok(aiGetInstruction(p, "")), nil
			}
			return fail(errors.New(out)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_catalog",
			mcp.WithDescription("List secret paths with descriptions and types (values never shown). "+
				"Metadata comes from the _meta block of each .ward file."),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, err := run(dirArgs(req.GetString("dir", ""), "catalog", "--json")...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_tree",
			mcp.WithDescription("Show merged tree with source file and line for each value. "+
				"In AI mode values are hidden as <sensitive> (strict) or masked, e.g. 'sk_l…a9f2 (51 chars)' (masked, the default)."),
			mcp.WithString("path", mcp.Description("optional scope: 'vault1:group.key1' (one vault) or 'group.key1' (overlay)")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"tree", "--ai-mode"}
			if p := req.GetString("path", ""); p != "" {
				args = append(args, p)
			}
			out, err := run(dirArgs(req.GetString("dir", ""), args...)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_envs",
			mcp.WithDescription("Show environment variables that would be injected by ward exec. "+
				"In AI mode values are hidden as <sensitive> (strict) or masked, e.g. 'sk_l…a9f2 (51 chars)' (masked, the default)."),
			mcp.WithString("path", mcp.Description("optional scope to select env vars; note: 'ward secrets' is the canonical command name now")),
			mcp.WithBoolean("prefixed", mcp.Description("use full dot-path names as env var keys")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"envs", "--ai-mode"}
			if req.GetBool("prefixed", false) {
				args = append(args, "--prefixed")
			}
			if p := req.GetString("path", ""); p != "" {
				args = append(args, p)
			}
			out, err := run(dirArgs(req.GetString("dir", ""), args...)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_raw",
			mcp.WithDescription("Show the raw (decrypted) contents of a .ward file (all files when none given). "+
				"Always refused through MCP: AI mode never exposes decrypted contents."),
			mcp.WithString("file", mcp.Description("optional path to a .ward file")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"raw"}
			if f := req.GetString("file", ""); f != "" {
				args = append(args, f)
			}
			out, err := run(dirArgs(req.GetString("dir", ""), args...)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_inspect",
			mcp.WithDescription("Inspect a .ward file showing encryption metadata"),
			mcp.WithString("file", mcp.Description("optional path to a .ward file")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"inspect"}
			if f := req.GetString("file", ""); f != "" {
				args = append(args, f)
			}
			out, err := run(dirArgs(req.GetString("dir", ""), args...)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_vaults",
			mcp.WithDescription("List configured vault paths from .ward/config.yaml"),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, err := run(dirArgs(req.GetString("dir", ""), "vaults")...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_exec",
			mcp.WithDescription("Execute a command with ward secrets injected as env vars. "+
				"Optionally scope which secrets are injected with 'scope' — a vault-qualified "+
				"path 'vault1:group.key1' (one vault) or 'group.key1' (overlay across all "+
				"vaults). Space-separate multiple scopes. See ward_docs."),
			mcp.WithString("command", mcp.Required(), mcp.Description("command and arguments, e.g. 'rails server'")),
			mcp.WithString("scope", mcp.Description("scope(s) to inject: 'vault1:group.key1' (one vault) "+
				"or 'group.key1' (overlay all vaults); space-separate multiple")),
			mcp.WithBoolean("prefixed", mcp.Description("use full dot-path names as env var keys")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var scopes []string
			if s := strings.TrimSpace(req.GetString("scope", "")); s != "" {
				scopes = strings.Fields(s)
			}
			args := execArgs(req.GetString("dir", ""), scopes, req.GetBool("prefixed", false), req.GetString("command", ""))
			out, err := run(args...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_export",
			mcp.WithDescription("Export merged secrets as shell export statements. "+
				"Refused in AI mode: decrypted contents are never printed."),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, err := run(dirArgs(req.GetString("dir", ""), "export")...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_import",
			mcp.WithDescription("Read YAML from content and encrypt it into the given .ward file"),
			mcp.WithString("file", mcp.Required(), mcp.Description("path to the .ward file")),
			mcp.WithString("content", mcp.Required(), mcp.Description("YAML content to encrypt into the file")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			file := req.GetString("file", "")
			content := req.GetString("content", "")
			out, err := runWithStdin(content, dirArgs(req.GetString("dir", ""), "import", file)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_file_add",
			mcp.WithDescription("Store a file's raw content as a single encrypted secret under the given vault and optional subdir"),
			mcp.WithString("filename", mcp.Required(), mcp.Description("original filename including extension, e.g. service-account.json")),
			mcp.WithString("content", mcp.Required(), mcp.Description("raw file content to store")),
			mcp.WithString("vault", mcp.Required(), mcp.Description("vault name and optional subdir, e.g. app or app.credentials")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filename := req.GetString("filename", "")
			content := req.GetString("content", "")
			vault := req.GetString("vault", "")

			tmp, err := os.CreateTemp("", filename)
			if err != nil {
				return fail(err), nil
			}
			defer os.Remove(tmp.Name())
			if _, err := tmp.WriteString(content); err != nil {
				return fail(err), nil
			}
			tmp.Close()

			out, err := run(dirArgs(req.GetString("dir", ""), "file", "add", tmp.Name(), vault)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_file_extract",
			mcp.WithDescription("Retrieve a file secret's raw content by original filename. "+
				"In AI mode the content is never returned: the tool replies with the shell command to run instead."),
			mcp.WithString("filename", mcp.Required(), mcp.Description("original filename including extension, e.g. service-account.json")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filename := req.GetString("filename", "")
			// AI mode: file secrets are never exposed in context — point the
			// agent at the bash command that writes the file to disk.
			return ok(aiFileInstruction(filename)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_set",
			mcp.WithDescription("Set a single secret at a full dot-path, from a literal 'value' or by copying another stored secret with 'from' (the copied value is never shown). Pass exactly one of value or from."),
			mcp.WithString("path", mcp.Required(), mcp.Description("scope of the secret to set: 'vault1:group.key1' (colon qualifies the vault)")),
			mcp.WithString("value", mcp.Description("value to store as-is (never read from a file or stdin; '@' and '-' are literal). Exclusive with from")),
			mcp.WithString("from", mcp.Description("scope of another leaf secret whose stored value is copied to path, across vaults too. Exclusive with value")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := req.GetString("path", "")
			args := req.GetArguments()
			_, hasValue := args["value"]
			from := req.GetString("from", "")
			if hasValue == (from != "") {
				return fail(errors.New("pass exactly one of 'value' or 'from'")), nil
			}
			setArgs := []string{"set", "--literal", "--", path, req.GetString("value", "")}
			if from != "" {
				setArgs = []string{"set", "--from", from, "--", path}
			}
			out, err := run(dirArgs(req.GetString("dir", ""), setArgs...)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_check",
			mcp.WithDescription("Check whether the contents of a file equal the stored secret, without revealing either. "+
				"Replies 'match' or 'differs'. Use it to confirm a value held outside ward is the right one."),
			mcp.WithString("path", mcp.Required(), mcp.Description("scope of the secret to compare against: 'vault1:group.key1'")),
			mcp.WithString("value_file", mcp.Required(), mcp.Description("path of a file holding the value to compare (absolute recommended; one trailing newline is ignored). A literal value is not accepted")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := req.GetString("path", "")
			valueFile := req.GetString("value_file", "")
			out, code, err := runExit(dirArgs(req.GetString("dir", ""), "check", "--", path, "@"+valueFile)...)
			if err != nil {
				return fail(err), nil
			}
			switch code {
			case 0:
				return ok("match"), nil
			case 1:
				return ok("differs"), nil
			}
			return fail(errors.New(out)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_unset",
			mcp.WithDescription("Remove a single secret at a full dot-path"),
			mcp.WithString("path", mcp.Required(), mcp.Description("scope of the secret to remove: 'vault1:group.key1'")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := req.GetString("path", "")
			out, err := run(dirArgs(req.GetString("dir", ""), "unset", path)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_new",
			mcp.WithDescription("Create a new vault (encrypted .ward file)"),
			mcp.WithString("name", mcp.Required(), mcp.Description("vault name, e.g. staging")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name := req.GetString("name", "")
			out, err := run(dirArgs(req.GetString("dir", ""), "new", name)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_config",
			mcp.WithDescription("Read or write a ward configuration value"),
			mcp.WithString("key", mcp.Required(), mcp.Description("config key")),
			mcp.WithString("value", mcp.Description("new value (omit to read current value)")),
			mcp.WithString("dir", mcp.Description("project directory containing .ward/config.yaml (default: current directory)")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"config", req.GetString("key", "")}
			if v := req.GetString("value", ""); v != "" {
				args = append(args, v)
			}
			out, err := run(dirArgs(req.GetString("dir", ""), args...)...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	return server.ServeStdio(s)
}

const wardDocs = `# ward — hierarchical secrets manager

ward merges encrypted YAML files (.ward) into a single secrets tree, resolved by dot-path.
Files are encrypted with age keys. The key lives in .ward/<vault>.key (local) or WARD_KEY (CI).

## Core concepts

- **vault**: a directory of .ward files (e.g. .ward/vault/)
- **scope**: e.g. vault1:group.key1 or group.key1 — selects part of the merged tree (see below)
- **merge**: files at deeper ancestry levels override parent values (child wins)
- **config**: .ward/config.yaml defines vaults and key location
- **.plain.ward**: structured plaintext file (never encrypted), read without a key

## Scope (vault-qualified paths)

A scope selects which part of the merged tree a command acts on:

- vault1:group.key1 — one vault (strict). Only that vault's subtree.
- group.key1 — no vault. For exec/envs/secrets/tree this OVERLAYS the same
  secret-path under EVERY vault, unioned (e.g. vault1 + vault2); genuine
  same-name collisions across vaults still error. For get (single value) it is a
  single lookup — ambiguous across vaults is an error.
- multiple scopes — space-separate to union explicit subtrees (exec/secrets).
- ns::sub:group.key1 — a sub-vault (e.g. myfish::restricted:secrets.production).
  A bare namespace (myfish:) is the default sub-vault (same as myfish::default:)
  and never includes its other sub-vaults.

A plain dotted path like vault1.group.key1 is treated as a secret-path, NOT as
vault vault1 — to target a vault use the colon form vault1:group.key1.

## Targeting a project (IMPORTANT for agents)

By default ward resolves the config from the current working directory. When working across
multiple projects, always pass the project directory so ward uses the right config:

` + "```" + `sh
ward --dir /path/to/project get DATABASE_URL   # by project root (recommended)
ward -d /path/to/project envs                  # shorthand
ward --config /path/to/project/.ward/config.yaml get DATABASE_URL  # explicit config file
` + "```" + `

All MCP tools accept an optional **dir** parameter for the same purpose:

` + "```" + `json
{ "tool": "ward_get", "path": "DATABASE_URL", "dir": "/path/to/project" }
` + "```" + `

Use **dir** whenever the agent is not already inside the target project directory.
Omit it only when the MCP server was started from inside the project root.

## AI mode — secrets are never exposed in context

ward runs in **AI mode** when the --ai-mode flag is passed, when the WARD_AI_MODE=1
env var is set, when ward detects an agent in its parent process chain (CLI calls
from a bash tool), or automatically for the MCP tools below. In AI mode secret
**values** are never returned to the agent:

- AI mode has two levels, set with ai_mode: strict|masked in .ward/config.yaml (or
  WARD_AI_MODE=strict|masked). The default is **masked**.
  - strict: values are shown as "<sensitive>".
  - masked: a single-line value shows only its first and last characters and its
    length, e.g. "tok_…a9f2 (51 chars)" — enough to confirm it is the right value.
    Values under 8 characters show only the length; multi-line values stay
    "<sensitive>"; a TOTP secret shows its masked seed, never the code.
- ward_get returns instructions for using the secret and, at the masked level, a
  "masked" field with the masked value. ward_file_extract replies with the
  **shell command** to run instead, so the agent can execute it in bash without
  ever reading the content.
- ward_tree and ward_envs show keys/paths with values hidden or masked.
- ward_catalog lists secret paths with descriptions/types — values are never shown,
  making it the safe way to discover what secrets exist.

If a value is needed, EXECUTE a command that consumes it — never read it:

` + "```" + `sh
ward exec -- sh -c 'curl -H "Authorization: Bearer $API_KEY" https://api.example.com'
ward exec -- sh -c 'psql "$DATABASE_URL" -c "select 1"'
` + "```" + `

ward get (strict level; at masked it prints the masked value), ward file extract,
ward raw, ward export <file> (to stdout) and --json refuse in AI mode (exit status 3): they exist to read a value, which must never
enter agent context. ward export <file> <out> writes to disk and is allowed.

To pin AI mode for the CLI too, set WARD_AI_MODE=1 in your agent's environment
(e.g. OpenCode opencode.json → "env": { "WARD_AI_MODE": "1" }, or Claude Code
.claude.json → same).

## Key commands (also available as MCP tools)

` + "```" + `sh
ward get [dot-path]          # merged value at path (or full tree)
ward tree [dot-path]         # merged tree with source file and line per value
ward catalog [--json]        # list secret paths with descriptions (values never shown)
ward secrets [scope]      # (envs is the deprecated alias)
ward raw [file]              # decrypted raw YAML of a .ward file (all files when none given)
ward inspect [dot-path]      # ancestry chain showing where each value comes from
ward vaults                  # list all configured vault paths
ward exec [scope] -- cmd     # run command with secrets injected as env vars
ward export [dot-path]       # export as shell export statements
ward check <scope> <value|@file|->  # match/differs against the stored secret (exit 0/1, errors 2); works in AI mode, MCP: ward_check
ward set <dot-path> <value>  # set a single secret at a full dot-path
ward set <dst> --from <src>  # copy a stored secret to another path/vault without revealing it (MCP: ward_set with from)
ward unset <dot-path>        # remove a single secret at a full dot-path
ward import <file.ward>      # read YAML from stdin and encrypt into the given .ward file
ward file add <f> <vault>    # store a file as a single encrypted secret (e.g. sa.json → app)
ward file extract <filename> # restore a file secret to disk by original filename
ward new <name>              # create a new .ward file
ward config <key> [value]    # read or write ward configuration
` + "```" + `

## Project setup

` + "```" + `sh
ward init                    # creates .ward/config.yaml, .ward/<vault>.key, first vault file
# ward init also adds .ward/<vault>.key to .gitignore automatically
` + "```" + `

## Multiple vaults (monorepo)

` + "```" + `yaml
# .ward/config.yaml
vaults:
  - name: vault1
    path: ./.ward/vault
  - name: vault2
    path: ../.other/ward/vaults/shared
` + "```" + `

Each vault can use its own encryption key. Resolution order per vault (highest priority first):

1. ` + "`" + `WARD_KEY_<NAME>` + "`" + ` env var (e.g. ` + "`" + `WARD_KEY_VAULT2` + "`" + `)
2. ` + "`" + `.ward/<name>.key` + "`" + ` file — auto-detected when present
3. Global ` + "`" + `WARD_KEY` + "`" + ` env var / ` + "`" + `encryption` + "`" + ` config

Sub-vaults use ` + "`" + `<namespace>::<name>` + "`" + ` (e.g. ` + "`" + `myfish::restricted` + "`" + `): env
` + "`" + `WARD_KEY_MYFISH_RESTRICTED` + "`" + `, key file ` + "`" + `.ward/myfish::restricted.key` + "`" + `. The default
sub-vault (` + "`" + `myfish` + "`" + ` = ` + "`" + `myfish::default` + "`" + `) accepts ` + "`" + `WARD_KEY_MYFISH` + "`" + ` and
` + "`" + `WARD_KEY_MYFISH_DEFAULT` + "`" + `.

` + "```" + `yaml
vaults:
  - name: vault1
    path: .ward/vaults/vault1
    encryption:
      key_file: .ward/vault1.key      # explicit key file
  - name: vault2
    path: ../.other/ward/vaults/vault2
    encryption:
      key_env: WARD_KEY_VAULT2      # explicit env var
` + "```" + `

` + "```" + `sh
# CI with multiple vault keys
WARD_KEY_VAULT1=ward-xxx WARD_KEY_VAULT2=ward-yyy ward envs
` + "```" + `

## Claude Code skills

- **/ward:handle-secrets** — use when handling ward secrets: create/edit vaults, scope commands (vault:secret-path), inject env vars, debug merge/key issues`

