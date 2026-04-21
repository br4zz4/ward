package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/oporpino/ward/internal/config"
	wardage "github.com/oporpino/ward/internal/age"
	"github.com/oporpino/ward/internal/secrets"
	"github.com/oporpino/ward/internal/sops"
	"github.com/oporpino/ward/internal/ward"
)

// configFile holds the explicit --config flag value; empty means auto-detect.
var configFile = ""

// resolvedConfig caches the config path after first resolution.
var resolvedConfig = ""

func SetConfigFile(path string) {
	configFile = path
	resolvedConfig = "" // reset cache
}

// resolvedConfigFile returns the config file path to use: explicit flag or auto-detected.
// The result is cached after the first successful resolution.
func resolvedConfigFile() (string, error) {
	if resolvedConfig != "" {
		return resolvedConfig, nil
	}
	if configFile != "" {
		resolvedConfig = configFile
		return resolvedConfig, nil
	}
	found, err := config.FindConfigFile()
	if err != nil {
		return "", err
	}
	resolvedConfig = found
	return resolvedConfig, nil
}

// newEngine loads the ward config and returns a ready-to-use Engine.
func newEngine() (*ward.Engine, error) {
	cfgPath, err := resolvedConfigFile()
	if err != nil {
		if isNotExistWrapped(err) {
			fmt.Fprintf(os.Stderr,
				"\n%s✗ not a ward project%s — %s not found\n\n"+
					"%sward%s organises secrets in layers using encrypted %s.ward%s files.\n"+
					"to get started, run:\n\n"+
					"  %sward init%s\n\n"+
					"this will create %s.ward/config.yaml%s and a starter secrets file.\n"+
					"%ssee https://github.com/oporpino/ward%s\n\n",
				clrLightRed, clrReset, config.DefaultConfigFile,
				clrBold, clrReset, clrCyan, clrReset,
				clrBold, clrReset,
				clrCyan, clrReset,
				clrGray, clrReset,
			)
			os.Exit(1)
		}
		return nil, fmt.Errorf("finding config: %w", err)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("loading %s: %w", cfgPath, err)
	}
	resolvedConfig = cfgPath // ensure cache is set
	dec, err := decryptorFor(cfg)
	if err != nil {
		return nil, err
	}
	return ward.NewEngine(cfg, dec), nil
}

// decryptorFor returns the appropriate Decryptor based on the config.
// Priority: WARD_KEY env var → key_env (user-defined env var) → key_file → MockDecryptor.
func decryptorFor(cfg *config.Config) (sops.Decryptor, error) {
	keyFile, err := resolveKeyFile(cfg)
	if err != nil {
		return nil, err
	}
	if keyFile == "" {
		return sops.MockDecryptor{}, nil
	}
	switch cfg.Encryption.Engine {
	case "sops+age":
		return sops.SopsDecryptor{KeyFile: keyFile}, nil
	case "age+armor", "":
		return wardage.AgeArmorDecryptor{KeyFile: keyFile}, nil
	default:
		return nil, fmt.Errorf("unknown encryption engine %q (supported: age+armor, sops+age)", cfg.Encryption.Engine)
	}
}

// resolveKeyFile resolves the age key file path from config/env, writing temp files as needed.
// Returns "" when no encryption is configured (plain files).
func resolveKeyFile(cfg *config.Config) (string, error) {
	// 1. WARD_KEY — portable base64 token, always checked first (CI-friendly)
	if token := os.Getenv("WARD_KEY"); token != "" {
		keyFile, err := writeTempKey(token)
		if err != nil {
			return "", fmt.Errorf("decoding WARD_KEY: %w", err)
		}
		return keyFile, nil
	}

	// 2. key_env — user-defined env var name containing raw age key content
	if cfg.Encryption.KeyEnv != "" {
		content := os.Getenv(cfg.Encryption.KeyEnv)
		if content == "" {
			fatalKeyError(
				fmt.Sprintf("env var %s%s%s is empty or not set", clrYellow, cfg.Encryption.KeyEnv, clrReset),
				fmt.Sprintf("set %s%s%s to the contents of your age key", clrYellow, cfg.Encryption.KeyEnv, clrReset),
			)
		}
		keyFile, err := writeTempKeyRaw([]byte(content))
		if err != nil {
			return "", fmt.Errorf("writing temp key from %s: %w", cfg.Encryption.KeyEnv, err)
		}
		return keyFile, nil
	}

	// 3. key_file
	if cfg.Encryption.KeyFile != "" {
		if _, err := os.Stat(cfg.Encryption.KeyFile); err != nil {
			fatalKeyError(
				fmt.Sprintf("key file %s%s%s not found", clrCyan, cfg.Encryption.KeyFile, clrReset),
				fmt.Sprintf("run %sward init%s to generate it, or copy your %s.ward.key%s here", clrBold, clrReset, clrCyan, clrReset),
			)
		}
		return cfg.Encryption.KeyFile, nil
	}

	return "", nil
}

// writeTempKey decodes a ward-<base64url> token and writes it to a temp file.
func writeTempKey(token string) (string, error) {
	data, err := decodeWardKey(token)
	if err != nil {
		return "", err
	}
	return writeTempKeyRaw(data)
}

// writeTempKeyRaw writes raw age key content to a temp file and returns its path.
func writeTempKeyRaw(data []byte) (string, error) {
	f, err := os.CreateTemp("", "ward-key-*")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// requireWardFile returns an error if path is not an existing .ward file.
func requireWardFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s: file not found", path)
	}
	if info.IsDir() {
		return fmt.Errorf("%s: is a directory — specify a .ward file", path)
	}
	return nil
}

// isNotExistWrapped unwraps err chain to check for os.ErrNotExist.
func isNotExistWrapped(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}

// fatal prints err to stderr and exits 1.
func fatal(err error) {
	fmt.Fprintln(os.Stderr, "ward:", err)
	os.Exit(1)
}

// fatalNoSources prints a styled error when no .ward file or source is configured.
func fatalNoSources() {
	fmt.Fprintf(os.Stderr,
		"\n  %s✗ no secrets file%s — no sources configured in %s.ward/config.yaml%s\n\n"+
			"  %s→%s create one with  %sward new staging%s\n\n",
		clrLightRed, clrReset, clrCyan, clrReset,
		clrGray, clrReset, clrBold, clrReset,
	)
	os.Exit(1)
}

// fatalKeyError prints a styled key-missing error and exits 1.
func fatalKeyError(problem, hint string) {
	fmt.Fprintf(os.Stderr,
		"\n  %s✗ no decryption key%s — %s\n\n  %s→%s %s\n\n",
		clrLightRed, clrReset, problem,
		clrGray, clrReset, hint,
	)
	os.Exit(1)
}

// --- ANSI colour constants ---------------------------------------------------

const (
	clrReset     = "\033[0m"
	clrBold      = "\033[1m"
	clrGray      = "\033[90m"
	clrCyan      = "\033[36m"
	clrYellow    = "\033[33m"
	clrLightRed  = "\033[91m"
	clrGreen     = "\033[32m"
	clrOrange    = "\033[38;5;208m"
)

// --- presentation ------------------------------------------------------------

// printTree renders a node as plain YAML-like text (used by get).
func printTree(node *secrets.Node, indent int) {
	prefix := strings.Repeat("  ", indent)
	if node.Children != nil {
		for _, k := range sortedKeys(node.Children) {
			child := node.Children[k]
			if child.Children != nil {
				fmt.Printf("%s%s:\n", prefix, k)
				printTree(child, indent+1)
			} else {
				fmt.Printf("%s%s: %v\n", prefix, k, child.Value)
			}
		}
	} else {
		fmt.Printf("%s%v\n", prefix, node.Value)
	}
}

// listLine is one rendered row for the aligned-origin display.
type listLine struct {
	text     string
	origin   string
	conflict bool
}

// printTreeWithOrigin renders the merged tree with colour-coded leaf origins.
// conflictKeys is the set of leaf key names in conflict (may be nil).
func printTreeWithOrigin(node *secrets.Node, indent int, anchorPath string, conflictKeys map[string]bool) {
	var lines []listLine
	collectListLines(node, indent, anchorPath, conflictKeys, &lines)

	maxLen := 0
	for _, l := range lines {
		if l.origin != "" && visibleLen(l.text) > maxLen {
			maxLen = visibleLen(l.text)
		}
	}

	for _, l := range lines {
		if l.origin != "" {
			padding := strings.Repeat(" ", maxLen-visibleLen(l.text)+2)
			arrow := clrYellow
			if l.conflict {
				arrow = clrLightRed
			}
			fmt.Printf("%s%s%s←%s %s\n", l.text, padding, arrow, clrReset, l.origin)
		} else {
			fmt.Println(l.text)
		}
	}

	if len(conflictKeys) > 0 {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides  %s●%s conflict%s\n",
			clrGray, clrGreen, clrGray, clrOrange, clrGray, clrLightRed, clrGray, clrReset)
	} else {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides%s\n",
			clrGray, clrGreen, clrGray, clrOrange, clrGray, clrReset)
	}
}

// --- tree traversal ----------------------------------------------------------

func collectListLines(node *secrets.Node, indent int, anchorPath string, conflictKeys map[string]bool, lines *[]listLine) {
	if node.Children == nil {
		return
	}
	prefix := strings.Repeat("  ", indent)

	var leafKeys, mapKeys []string
	for k, child := range node.Children {
		if child.Children != nil {
			mapKeys = append(mapKeys, k)
		} else {
			leafKeys = append(leafKeys, k)
		}
	}
	sort.Slice(leafKeys, func(i, j int) bool {
		ci, cj := node.Children[leafKeys[i]], node.Children[leafKeys[j]]
		pi, pj := leafPriority(ci, leafKeys[i], conflictKeys), leafPriority(cj, leafKeys[j], conflictKeys)
		if pi != pj {
			return pi < pj
		}
		return leafKeys[i] < leafKeys[j]
	})
	sort.Strings(mapKeys)

	for _, k := range leafKeys {
		child := node.Children[k]
		color := leafColor(child, k, conflictKeys)
		*lines = append(*lines, listLine{
			text:     fmt.Sprintf("%s%s%s:%s %s%v%s", prefix, color, k, clrReset, clrGray, child.Value, clrReset),
			origin:   formatOrigin(child.Origin),
			conflict: conflictKeys[k],
		})
	}
	for _, k := range mapKeys {
		child := node.Children[k]
		*lines = append(*lines, listLine{
			text: fmt.Sprintf("%s%s%s%s:", prefix, clrBold, k, clrReset),
		})
		collectListLines(child, indent+1, anchorPath, conflictKeys, lines)
	}
}

// leafPriority returns sort order: 0=conflict, 1=override, 2=active.
func leafPriority(child *secrets.Node, k string, conflictKeys map[string]bool) int {
	switch {
	case conflictKeys[k]:
		return 0
	case child.Overrides:
		return 1
	default:
		return 2
	}
}

func leafColor(child *secrets.Node, k string, conflictKeys map[string]bool) string {
	switch {
	case conflictKeys[k]:
		return clrLightRed
	case child.Overrides:
		return clrOrange
	default:
		return clrGreen
	}
}

func formatOrigin(o secrets.Origin) string {
	if o.File == "" {
		return ""
	}
	if o.Line > 0 {
		return fmt.Sprintf("%s%s%s:%s%d%s", clrCyan, o.File, clrReset, clrLightRed, o.Line, clrReset)
	}
	return fmt.Sprintf("%s%s%s", clrCyan, o.File, clrReset)
}

// --- utilities ---------------------------------------------------------------

// visibleLen returns the visible (non-ANSI) length of s.
func visibleLen(s string) int {
	n, inEsc := 0, false
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEsc = true
		}
		if inEsc {
			if s[i] == 'm' {
				inEsc = false
			}
			continue
		}
		n++
	}
	return n
}

// sortedKeys returns the keys of m sorted alphabetically.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
