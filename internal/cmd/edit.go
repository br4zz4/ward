package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/br4zz4/ward/internal/config"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/spf13/cobra"
)

func NewEditCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "edit [vault] [file|scope]",
		Short: "Decrypt a .ward file, open in $EDITOR, re-encrypt on save",
		Args:  cobra.MaximumNArgs(2),
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

// wardFilePath resolves the edit target from the command arguments:
//
//	(no args)          → ask which vault, then which file
//	<vault>            → ask which file within that vault
//	<vault> <path>     → the file at that path within the vault
//	<file|scope>       → a filesystem path, or a vault:secret-path scope
func wardFilePath(args []string) string {
	if len(args) == 0 {
		return pickVaultThenFile()
	}
	if len(args) == 2 {
		return fileInVault(args[0], args[1])
	}

	// Resolve user-supplied path relative to the original CWD (before
	// FindConfigFile changed directory to the project root).
	path := args[0]
	if !filepath.IsAbs(path) {
		if orig := OriginalDir(); orig != "" {
			path = filepath.Join(orig, path)
		}
	}
	// If path is a directory, resolve to a .ward file inside it.
	info, err := os.Stat(path)
	if err != nil {
		// A bare vault name selects that vault and asks which file.
		if src := vaultNamed(args[0]); src != nil {
			return pickFileInVault(src)
		}
		// Path doesn't exist — try to find it inside the vaults.
		if found := findInVaults(filepath.Base(path)); found != "" {
			return found
		}
		// Not a file reference — try it as a scope (vault:secret-path).
		if found := findByScope(args[0]); found != "" {
			return found
		}
		return path // let Decrypt report the original error
	}
	if info.IsDir() {
		return pickWardFile(path)
	}
	return path
}

// vaultNamed returns the configured vault with the given name, or nil.
func vaultNamed(name string) *config.Source {
	cfgPath, err := resolvedConfigFile()
	if err != nil {
		return nil
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil
	}
	return findVault(cfg, name)
}

// pickVaultThenFile asks which vault to edit (skipped when only one exists),
// then which file inside it.
func pickVaultThenFile() string {
	cfgPath, err := resolvedConfigFile()
	if err != nil {
		fatalNoSources()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fatal(err)
	}
	if len(cfg.Vaults) == 0 {
		fatalNoSources()
	}
	names := make([]string, len(cfg.Vaults))
	for i, v := range cfg.Vaults {
		names[i] = v.Name
	}
	name, err := promptChoice("Select a vault:", names, os.Stdin, os.Stdout)
	if err != nil {
		fatal(err)
	}
	return pickFileInVault(findVault(cfg, name))
}

// pickFileInVault lists every .ward file in the vault and asks which to edit.
func pickFileInVault(src *config.Source) string {
	files, err := secrets.Discover([]string{src.Path})
	if err != nil || len(files) == 0 {
		fatal(fmt.Errorf("no .ward files found in vault %s (%s)", src.Name, src.Path))
	}
	picked, err := pickFromList(files, os.Stdin, os.Stdout)
	if err != nil {
		fatal(err)
	}
	return picked
}

// fileInVault resolves arg to a file inside the named vault, prompting when
// the argument matches more than one.
func fileInVault(vaultName, arg string) string {
	src := vaultNamed(vaultName)
	if src == nil {
		fatalVaultNotFound(vaultName)
	}
	files, err := secrets.Discover([]string{src.Path})
	if err != nil {
		fatal(err)
	}
	matches := resolveVaultFile(src.Path, files, arg)
	if len(matches) == 0 {
		fatal(fmt.Errorf("no .ward file matching %q in vault %s (%s)", arg, src.Name, src.Path))
	}
	picked, err := pickFromList(matches, os.Stdin, os.Stdout)
	if err != nil {
		fatal(err)
	}
	return picked
}

// resolveVaultFile returns the files under vaultPath that arg addresses. The
// argument may use slashes or dots, carry the .ward extension or not, name a
// directory (selecting everything beneath it), or be a bare basename.
//
// The path is tried exactly as typed before dots are read as separators, so a
// file or directory whose name legitimately contains a dot stays addressable.
func resolveVaultFile(vaultPath string, files []string, arg string) []string {
	for _, rel := range vaultRelForms(arg) {
		if matches := matchVaultRel(vaultPath, files, rel); len(matches) > 0 {
			return matches
		}
	}
	return nil
}

// vaultRelForms returns the vault-relative paths arg may denote, most literal
// first: the path as typed, then the same with dots read as separators. Only a
// trailing .ward is stripped as an extension.
func vaultRelForms(arg string) []string {
	var forms []string
	literal := strings.Trim(strings.TrimSuffix(arg, ".ward"), "/")
	if literal != "" {
		forms = append(forms, literal)
	}
	dotted := strings.Trim(strings.TrimSuffix(strings.ReplaceAll(arg, ".", "/"), "/ward"), "/")
	if dotted != "" && dotted != literal {
		forms = append(forms, dotted)
	}
	return forms
}

// matchVaultRel resolves one vault-relative form against the vault's files:
// an exact file, else a directory prefix, else a bare basename.
func matchVaultRel(vaultPath string, files []string, rel string) []string {
	var exact, beneath, byBase []string
	for _, f := range files {
		switch relPath := vaultRelPath(vaultPath, f); {
		case relPath == rel+".ward":
			exact = append(exact, f)
		case strings.HasPrefix(relPath, rel+"/"):
			beneath = append(beneath, f)
		}
		if strings.TrimSuffix(filepath.Base(f), ".ward") == rel {
			byBase = append(byBase, f)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	if len(beneath) > 0 {
		return beneath
	}
	return byBase
}

// vaultRelPath returns file's path relative to the vault root, in slash form.
func vaultRelPath(vaultPath, file string) string {
	rel, err := filepath.Rel(vaultPath, file)
	if err != nil {
		return file
	}
	return filepath.ToSlash(rel)
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
// unqualified one overlays the secret-path under each file's vault root, so
// "main.production" finds it inside any vault.
//
// Parsed trees are rooted by vault name, so an unqualified path is never
// matched against the root itself — that would make a plain dot identify a
// vault, which the scope rules forbid (see docs/hierarchy.md).
func scopeTargetFiles(files []secrets.ParsedFile, sc secrets.Scope) []string {
	if sc.Vault != "" {
		return secrets.FilesMatching(files, sc.TreePath(), secrets.Exists)
	}
	if sc.SecretPath == "" {
		return nil
	}
	var out []string
	for _, pf := range files {
		tree := secrets.NewTree(pf.Data)
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

// pickFromList prompts for one of files, shallowest path first.
func pickFromList(files []string, in io.Reader, out io.Writer) (string, error) {
	sort.Slice(files, func(i, j int) bool {
		di, dj := strings.Count(files[i], "/"), strings.Count(files[j], "/")
		if di != dj {
			return di < dj
		}
		return files[i] < files[j]
	})
	return promptChoice("Select a file to edit:", files, in, out)
}

// promptChoice prints label and the numbered items on out, then reads the
// selection from in. Items are offered in the order given. A single item is
// returned directly, without prompting.
func promptChoice(label string, items []string, in io.Reader, out io.Writer) (string, error) {
	switch len(items) {
	case 0:
		return "", fmt.Errorf("nothing to choose from")
	case 1:
		return items[0], nil
	}
	fmt.Fprintln(out, label)
	for i, it := range items {
		fmt.Fprintf(out, "  %d) %s\n", i+1, it)
	}
	fmt.Fprint(out, "> ")
	var choice int
	if _, err := fmt.Fscan(in, &choice); err != nil || choice < 1 || choice > len(items) {
		return "", fmt.Errorf("invalid choice")
	}
	return items[choice-1], nil
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
