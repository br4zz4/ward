package mcp

import (
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
		mcp.NewTool("ward_view",
			mcp.WithDescription("Show merged tree with source file and line for each value"),
			mcp.WithString("path", mcp.Description("optional dot-path to scope the view")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := []string{"view"}
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
			mcp.WithDescription("Show the raw (decrypted) contents of a .ward file"),
			mcp.WithString("file", mcp.Required(), mcp.Description("path to the .ward file")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			file := req.GetString("file", "")
			out, err := run("raw", file)
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
		mcp.NewTool("ward_override",
			mcp.WithDescription("Set a runtime override for a secret value"),
			mcp.WithString("path", mcp.Required(), mcp.Description("dot-path of the secret")),
			mcp.WithString("value", mcp.Required(), mcp.Description("new value")),
		),
		func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := req.GetString("path", "")
			value := req.GetString("value", "")
			out, err := run("override", path, value)
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
ward view [dot-path]         # merged tree with source file and line per value
ward envs [dot-path]         # env vars that would be injected by ward exec
ward raw <file>              # decrypted raw YAML of a .ward file
ward inspect [dot-path]      # ancestry chain showing where each value comes from
ward vaults                  # list all configured vault paths
ward exec <dot-path> -- cmd  # run command with secrets injected as env vars
ward export [dot-path]       # export as shell export statements
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

