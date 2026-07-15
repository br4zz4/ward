package mcp

import (
	"bytes"
	"context"
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

func run(args ...string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		bin = "ward"
	}
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	text := stripANSI(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("%s", text)
	}
	return text, nil
}

func runWithStdin(stdin string, args ...string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		bin = "ward"
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewBufferString(stdin)
	out, err := cmd.CombinedOutput()
	text := stripANSI(strings.TrimSpace(string(out)))
	if err != nil {
		return "", fmt.Errorf("%s", text)
	}
	return text, nil
}

func ok(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

func fail(err error) *mcp.CallToolResult {
	return mcp.NewToolResultError(err.Error())
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
			mcp.WithDescription("Return the merged value at a dot-path (or full tree if no path given)"),
			mcp.WithString("path", mcp.Description("dot-path to a secret, e.g. project.staging.secret_key")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"get"}
			if p := req.GetString("path", ""); p != "" {
				args = append(args, p)
			}
			out, err := run(args...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_tree",
			mcp.WithDescription("Show merged tree with source file and line for each value"),
			mcp.WithString("path", mcp.Description("optional dot-path to scope the view")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"tree"}
			if p := req.GetString("path", ""); p != "" {
				args = append(args, p)
			}
			out, err := run(args...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_envs",
			mcp.WithDescription("Show environment variables that would be injected by ward exec"),
			mcp.WithString("path", mcp.Description("optional dot-path to scope env vars")),
			mcp.WithBoolean("prefixed", mcp.Description("use full dot-path names as env var keys")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"envs"}
			if req.GetBool("prefixed", false) {
				args = append(args, "--prefixed")
			}
			if p := req.GetString("path", ""); p != "" {
				args = append(args, p)
			}
			out, err := run(args...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_raw",
			mcp.WithDescription("Show the raw (decrypted) contents of a .ward file (all files when none given)"),
			mcp.WithString("file", mcp.Description("optional path to a .ward file")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"raw"}
			if f := req.GetString("file", ""); f != "" {
				args = append(args, f)
			}
			out, err := run(args...)
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
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"inspect"}
			if f := req.GetString("file", ""); f != "" {
				args = append(args, f)
			}
			out, err := run(args...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_vaults",
			mcp.WithDescription("List configured vault paths from .ward/config.yaml"),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, err := run("vaults")
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_exec",
			mcp.WithDescription("Execute a command with ward secrets injected as environment variables"),
			mcp.WithString("command", mcp.Required(), mcp.Description("command and arguments to run, e.g. 'rails server'")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			command := req.GetString("command", "")
			parts := strings.Fields(command)
			args := append([]string{"exec", "--"}, parts...)
			out, err := run(args...)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_export",
			mcp.WithDescription("Export merged secrets as shell export statements"),
		),
		func(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			out, err := run("export")
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
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			file := req.GetString("file", "")
			content := req.GetString("content", "")
			out, err := runWithStdin(content, "import", file)
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

			out, err := run("file", "add", tmp.Name(), vault)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_file_extract",
			mcp.WithDescription("Retrieve a file secret's raw content by original filename"),
			mcp.WithString("filename", mcp.Required(), mcp.Description("original filename including extension, e.g. service-account.json")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			filename := req.GetString("filename", "")

			tmp, err := os.MkdirTemp("", "ward-file-extract-*")
			if err != nil {
				return fail(err), nil
			}
			defer os.RemoveAll(tmp)

			if _, err := run("file", "extract", filename, tmp); err != nil {
				return fail(err), nil
			}
			data, err := os.ReadFile(tmp + "/" + filename)
			if err != nil {
				return fail(err), nil
			}
			return ok(string(data)), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_set",
			mcp.WithDescription("Set a single secret at a full dot-path"),
			mcp.WithString("path", mcp.Required(), mcp.Description("full dot-path of the secret, e.g. myapp.staging.secret_key")),
			mcp.WithString("value", mcp.Required(), mcp.Description("value to set")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := req.GetString("path", "")
			value := req.GetString("value", "")
			out, err := run("set", path, value)
			if err != nil {
				return fail(err), nil
			}
			return ok(out), nil
		},
	)

	s.AddTool(
		mcp.NewTool("ward_unset",
			mcp.WithDescription("Remove a single secret at a full dot-path"),
			mcp.WithString("path", mcp.Required(), mcp.Description("full dot-path of the secret to remove")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := req.GetString("path", "")
			out, err := run("unset", path)
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
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name := req.GetString("name", "")
			out, err := run("new", name)
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
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"config", req.GetString("key", "")}
			if v := req.GetString("value", ""); v != "" {
				args = append(args, v)
			}
			out, err := run(args...)
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
Files are encrypted with age keys. The key lives in .ward.key (local) or WARD_KEY (CI).

## Core concepts

- **vault**: a directory of .ward files (e.g. .ward/vault/)
- **dot-path**: e.g. myapp.environments.staging — addresses a node in the merged tree
- **merge**: files at deeper ancestry levels override parent values (child wins)
- **config**: .ward/config.yaml defines vaults and key location

## Key commands (also available as MCP tools)

` + "```" + `sh
ward get [dot-path]          # merged value at path (or full tree)
ward tree [dot-path]         # merged tree with source file and line per value
ward envs [dot-path]         # env vars that would be injected by ward exec
ward raw [file]              # decrypted raw YAML of a .ward file (all files when none given)
ward inspect [dot-path]      # ancestry chain showing where each value comes from
ward vaults                  # list all configured vault paths
ward exec <dot-path> -- cmd  # run command with secrets injected as env vars
ward export [dot-path]       # export as shell export statements
ward set <dot-path> <value>  # set a single secret at a full dot-path
ward unset <dot-path>        # remove a single secret at a full dot-path
ward import <file.ward>      # read YAML from stdin and encrypt into the given .ward file
ward file add <f> <vault> # store a file as a single encrypted secret (e.g. sa.json → app)
ward file extract <filename>  # restore a file secret to disk by original filename
ward new <name>              # create a new .ward file
ward config <key> [value]    # read or write ward configuration
` + "```" + `

## Project setup

` + "```" + `sh
ward init                    # creates .ward/config.yaml, .ward.key, first vault file
echo ".ward.key" >> .gitignore
` + "```" + `

## Multiple vaults (monorepo)

` + "```" + `yaml
# .ward/config.yaml
vaults:
  - path: ./.ward/vault
  - path: ../.commons/ward/vaults/shared
` + "```" + `

## Claude Code skills

- **/ward:context** — use when working with ward in a project (setup, vaults, debugging)`

