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

// spaces returns a string of n space characters.
func spaces(n int) string {
	if n <= 0 {
		return " "
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
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
	clrReset      = "\033[0m"
	clrBold       = "\033[1m"
	clrDim        = "\033[2m"
	clrGray       = "\033[90m"         // dark gray — ghosted text
	clrGrayLight  = "\033[37m"         // light gray — normal values
	clrCyan       = "\033[36m"         // cyan — normal file paths
	clrCyanDim    = "\033[2;36m"       // dim cyan — ghosted file path (unused, kept for ref)
	clrYellow     = "\033[33m"
	clrLightRed   = "\033[91m"
	clrRedDim     = "\033[2;31m"       // dim red — conflict arrow
	clrMagentaSoft = "\033[38;5;133m"  // soft magenta — conflict winner line number
	clrGreen      = "\033[32m"
	clrOrange     = "\033[38;5;208m"
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
	text        string
	originFile  string // file path (uncolored)
	originLine  int    // line number (0 = no line)
	conflict    bool   // file-level conflict (same dot-path, multiple files)
	envConflict bool   // env var collision (different dot-paths, same leaf name)
	overrides   bool   // shadowed by a deeper leaf with same key name
	extra       bool   // ghosted secondary source line
}

// printTreeWithOrigin renders the merged tree with colour-coded leaf origins.
// conflicts maps dot-path → Conflict; envCollisions marks dot-paths with env var collisions.
func printTreeWithOrigin(node *secrets.Node, indent int, conflicts map[string]secrets.Conflict, prefix string, envCollisions map[string]bool) {
	var lines []listLine
	collectListLines(node, indent, conflicts, prefix, envCollisions, &lines)

	maxLen := 0
	for _, l := range lines {
		if l.originFile != "" && visibleLen(l.text) > maxLen {
			maxLen = visibleLen(l.text)
		}
	}

	for _, l := range lines {
		if l.originFile != "" {
			vl := visibleLen(l.text)
			pad := maxLen - vl + 6
			if pad < 1 {
				pad = 1
			}
			padding := strings.Repeat(" ", pad)

			// Status color drives arrow, file path, and line number.
			statusClr := clrMagentaSoft // active
			lineClr := clrMagentaSoft
			if l.extra {
				statusClr = clrGray
				lineClr = clrGray
			} else if l.conflict {
				statusClr = clrMagentaSoft // winner of file conflict is still "active"
				lineClr = clrMagentaSoft
			} else if l.envConflict {
				statusClr = clrLightRed
				lineClr = clrLightRed
			} else if l.overrides {
				statusClr = clrOrange
				lineClr = clrOrange
			}

			var originStr string
			if l.extra {
				originStr = fmt.Sprintf("%s%s:%d%s", clrGray, l.originFile, l.originLine, clrReset)
			} else if l.originLine > 0 {
				originStr = fmt.Sprintf("%s%s%s:%s%d%s", clrCyan, l.originFile, clrReset, lineClr, l.originLine, clrReset)
			} else {
				originStr = fmt.Sprintf("%s%s%s", clrCyan, l.originFile, clrReset)
			}

			fmt.Printf("%s%s%s←%s %s\n", l.text, padding, statusClr, clrReset, originStr)
		} else {
			fmt.Println(l.text)
		}
	}

	hasConflict := len(conflicts) > 0 || len(envCollisions) > 0
	if hasConflict {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides  %s●%s conflict  %s●%s ghosted%s\n",
			clrGray, clrGreen, clrGray, clrOrange, clrGray, clrLightRed, clrGray, clrGray, clrGray, clrReset)
	} else {
		fmt.Printf("\n%s%s●%s active  %s●%s overrides%s\n",
			clrGray, clrGreen, clrGray, clrOrange, clrGray, clrReset)
	}
}

// --- tree traversal ----------------------------------------------------------

func collectListLines(node *secrets.Node, indent int, conflicts map[string]secrets.Conflict, dotPrefix string, envCollisions map[string]bool, lines *[]listLine) {
	if node.Children == nil {
		return
	}
	indentStr := strings.Repeat("  ", indent)

	var leafKeys, mapKeys []string
	for k, child := range node.Children {
		if child.Children != nil {
			mapKeys = append(mapKeys, k)
		} else {
			leafKeys = append(leafKeys, k)
		}
	}
	sort.Slice(leafKeys, func(i, j int) bool {
		dp1 := dotJoin(dotPrefix, leafKeys[i])
		dp2 := dotJoin(dotPrefix, leafKeys[j])
		_, ci := conflicts[dp1]
		_, cj := conflicts[dp2]
		ci = ci || envCollisions[dp1]
		cj = cj || envCollisions[dp2]
		ni, nj := node.Children[leafKeys[i]], node.Children[leafKeys[j]]
		pi := leafPriorityConflict(ni, ci)
		pj := leafPriorityConflict(nj, cj)
		if pi != pj {
			return pi < pj
		}
		return leafKeys[i] < leafKeys[j]
	})
	sort.Strings(mapKeys)

	for _, k := range leafKeys {
		child := node.Children[k]
		dp := dotJoin(dotPrefix, k)
		if c, isConflict := conflicts[dp]; isConflict {
			// Winner: key green, value light gray
			last := c.Sources[len(c.Sources)-1]
			*lines = append(*lines, listLine{
				text:        fmt.Sprintf("%s%s%s:%s %s%v%s", indentStr, clrGreen, k, clrReset, clrGrayLight, child.Value, clrReset),
				originFile:  last.File,
				originLine:  last.Line,
				conflict:    true,
			})
			// Ghosted: sources that lost
			for _, src := range c.Sources[:len(c.Sources)-1] {
				snippet := src.Snippet
				if snippet == "" {
					snippet = src.File
				}
				*lines = append(*lines, listLine{
					text:       fmt.Sprintf("%s%s%s%s", indentStr, clrGray, snippet, clrReset),
					originFile: src.File,
					originLine: src.Line,
					conflict:   true,
					extra:      true,
				})
			}
		} else {
			isEnvConflict := envCollisions[dp]
			isOverrides := child.Overrides && !isEnvConflict
			color := clrGreen
			if isEnvConflict {
				color = clrLightRed
			} else if isOverrides {
				color = clrOrange
			}
			*lines = append(*lines, listLine{
				text:        fmt.Sprintf("%s%s%s:%s %s%v%s", indentStr, color, k, clrReset, clrGrayLight, child.Value, clrReset),
				originFile:  child.Origin.File,
				originLine:  child.Origin.Line,
				envConflict: isEnvConflict,
				overrides:   isOverrides,
			})
		}
	}
	for _, k := range mapKeys {
		child := node.Children[k]
		*lines = append(*lines, listLine{
			text: fmt.Sprintf("%s%s%s%s:", indentStr, clrBold, k, clrReset),
		})
		collectListLines(child, indent+1, conflicts, dotJoin(dotPrefix, k), envCollisions, lines)
	}
}

func dotJoin(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// leafPriorityConflict returns sort order: 0=conflict, 1=override, 2=active.
func leafPriorityConflict(child *secrets.Node, isConflict bool) int {
	switch {
	case isConflict:
		return 0
	case child.Overrides:
		return 1
	default:
		return 2
	}
}

func formatOrigin(o secrets.Origin) string {
	if o.File == "" {
		return ""
	}
	if o.Line > 0 {
		return fmt.Sprintf("%s%s%s:%s%d%s", clrCyan, o.File, clrReset, clrGreen, o.Line, clrReset)
	}
	return fmt.Sprintf("%s%s%s", clrCyan, o.File, clrReset)
}

// formatOriginDim renders origin in muted gray (for overridden/shadowed nodes).
func formatOriginDim(o secrets.Origin) string {
	if o.File == "" {
		return ""
	}
	if o.Line > 0 {
		return fmt.Sprintf("%s%s:%d%s", clrGray, o.File, o.Line, clrReset)
	}
	return fmt.Sprintf("%s%s%s", clrGray, o.File, clrReset)
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
