package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewEditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "edit [file.ward]",
		Short: "Decrypt a .ward file, open in $EDITOR, re-encrypt on save",
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: completeWardFiles,
		Run: func(_ *cobra.Command, args []string) {
			path := wardFilePath(args)

			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			plain, err := eng.Decrypt(path)
			if err != nil {
				fatal(fmt.Errorf("decrypting %s: %w", path, err))
			}

			tmp, err := writeTempFile(path, plain)
			if err != nil {
				fatal(err)
			}
			defer os.Remove(tmp)

			if err := openEditor(tmp); err != nil {
				fatal(err)
			}

			edited, err := os.ReadFile(tmp)
			if err != nil {
				fatal(fmt.Errorf("reading temp file: %w", err))
			}

			if err := eng.Encrypt(path, edited); err != nil {
				fatal(fmt.Errorf("re-encrypting %s: %w", path, err))
			}
		},
	}
	return c
}

func wardFilePath(args []string) string {
	var path string
	if len(args) == 1 {
		path = args[0]
		// Resolve user-supplied path relative to the original CWD (before
		// FindConfigFile changed directory to the project root).
		if !filepath.IsAbs(path) {
			if orig := OriginalDir(); orig != "" {
				path = filepath.Join(orig, path)
			}
		}
	} else {
		eng, err := newEngine()
		if err != nil {
			fatalNoSources()
		}
		sources := eng.SourcePaths()
		if len(sources) == 0 {
			fatalNoSources()
		}
		path = sources[0]
	}
	// If path is a directory, resolve to the first .ward file inside it.
	info, err := os.Stat(path)
	if err != nil {
		// Path doesn't exist — try to find it inside the vaults.
		if found := findInVaults(filepath.Base(path)); found != "" {
			return found
		}
		// Not a file reference — try it as a scope (vault:secret-path).
		if len(args) == 1 {
			if found := findByScope(args[0]); found != "" {
				return found
			}
		}
		return path // let Decrypt report the original error
	}
	if info.IsDir() {
		return pickWardFile(path)
	}
	return path
}

// findInVaults searches vault source directories for a .ward file whose path
// ends with the given suffix (e.g. "company.ward" or "secrets/company.ward").
func findInVaults(partial string) string {
	eng, err := newEngine()
	if err != nil {
		return ""
	}
	// Normalise: add .ward extension if missing
	if !strings.HasSuffix(partial, ".ward") {
		partial = partial + ".ward"
	}
	allFiles, err := secrets.Discover(eng.SourcePaths())
	if err != nil {
		return ""
	}
	// Exact suffix match first
	for _, f := range allFiles {
		if strings.HasSuffix(f, partial) || strings.HasSuffix(f, "/"+partial) {
			return f
		}
	}
	// Basename match (e.g. "company" matches ".ward/vault/company.ward")
	base := filepath.Base(partial)
	var matches []string
	for _, f := range allFiles {
		if filepath.Base(f) == base {
			matches = append(matches, f)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		return pickWardFile(filepath.Dir(matches[0])) // ambiguous — show picker
	}
	return ""
}

// findByScope resolves a scope argument (vault:secret-path or bare dot-path)
// to the .ward file defining it. When the scope's path spans several files,
// the user picks one. Returns "" when the argument matches nothing.
func findByScope(arg string) string {
	eng, err := newEngine()
	if err != nil {
		return ""
	}
	files, err := eng.LoadFiles()
	if err != nil {
		return ""
	}
	targets := scopeTargetFiles(files, secrets.ParseScope(arg))
	if len(targets) == 0 {
		return ""
	}
	picked, err := pickFromList(targets, os.Stdin, os.Stdout)
	if err != nil {
		fatal(err)
	}
	return picked
}

// scopeTargetFiles returns the .ward files whose data defines the scope's path
// (leaf or group). A qualified scope matches vault.secret-path directly; an
// unqualified one also overlays the secret-path under each file's root keys,
// so "main.production" finds it inside any vault.
func scopeTargetFiles(files []secrets.ParsedFile, sc secrets.Scope) []string {
	if sc.Vault != "" {
		return secrets.FilesMatching(files, sc.TreePath(), secrets.Exists)
	}
	var out []string
	for _, pf := range files {
		tree := secrets.NewTree(pf.Data)
		if tree.Kind(sc.SecretPath) != secrets.KindAbsent {
			out = append(out, pf.File)
			continue
		}
		for root := range pf.Data {
			if tree.Kind(root+"."+sc.SecretPath) != secrets.KindAbsent {
				out = append(out, pf.File)
				break
			}
		}
	}
	return out
}

// pickWardFile lists .ward files under dir and prompts the user to choose one.
func pickWardFile(dir string) string {
	files, err := secrets.Discover([]string{dir})
	if err != nil || len(files) == 0 {
		fatal(fmt.Errorf("no .ward files found in %s", dir))
	}
	picked, err := pickFromList(files, os.Stdin, os.Stdout)
	if err != nil {
		fatal(err)
	}
	return picked
}

// pickFromList prompts on out for one of files, reading the choice from in.
// A single candidate is returned directly without prompting.
func pickFromList(files []string, in io.Reader, out io.Writer) (string, error) {
	sort.Slice(files, func(i, j int) bool {
		di, dj := strings.Count(files[i], "/"), strings.Count(files[j], "/")
		if di != dj {
			return di < dj
		}
		return files[i] < files[j]
	})
	if len(files) == 1 {
		return files[0], nil
	}
	fmt.Fprintln(out, "Select a file to edit:")
	for i, f := range files {
		fmt.Fprintf(out, "  %d) %s\n", i+1, f)
	}
	fmt.Fprint(out, "> ")
	var choice int
	if _, err := fmt.Fscan(in, &choice); err != nil || choice < 1 || choice > len(files) {
		return "", fmt.Errorf("invalid choice")
	}
	return files[choice-1], nil
}

func writeTempFile(originalPath string, content []byte) (string, error) {
	ext := filepath.Ext(originalPath)
	tmp, err := os.CreateTemp("", "ward-edit-*"+ext)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", fmt.Errorf("writing temp file: %w", err)
	}
	tmp.Close()
	return tmp.Name(), nil
}

func openEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil // vim/neovim exit 1 for non-fatal warnings (swap files, netrw, etc)
		}
		return fmt.Errorf("editor exited with error: %w", err)
	}
	return nil
}
