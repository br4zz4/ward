package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"

	"github.com/br4zz4/ward/internal/config"
	wardage "github.com/br4zz4/ward/internal/age"
	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/sops"
	"github.com/br4zz4/ward/internal/ward"
)

// configFile holds the explicit --config flag value; empty means auto-detect.
var configFile = ""

// resolvedConfig caches the config path after first resolution.
var resolvedConfig = ""

// originalDir is the working directory at startup, before any chdir from FindConfigFile.
var originalDir = ""

func SetConfigFile(path string) {
	configFile = path
	resolvedConfig = "" // reset cache
}

// OriginalDir returns the working directory before ward changed to the project root.
func OriginalDir() string {
	return originalDir
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
	found, origDir, err := config.FindConfigFile()
	if err != nil {
		return "", err
	}
	resolvedConfig = found
	originalDir = origDir
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
					"%ssee https://github.com/br4zz4/ward%s\n\n",
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
	vaultDecs, err := BuildVaultDecryptors(cfg)
	if err != nil {
		return nil, err
	}
	globalDec, err := decryptorFor(cfg)
	if err != nil {
		return nil, err
	}
	return ward.NewEngineWithVaultDecryptors(cfg, globalDec, vaultDecs), nil
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

// BuildVaultDecryptors returns a map of vault name → Decryptor, one per vault.
// Each vault uses its own key when configured; otherwise falls back to the global decryptor.
func BuildVaultDecryptors(cfg *config.Config) (map[string]sops.Decryptor, error) {
	globalDec, err := decryptorFor(cfg)
	if err != nil {
		return nil, err
	}
	result := make(map[string]sops.Decryptor, len(cfg.Vaults))
	for _, v := range cfg.Vaults {
		dec, err := decryptorForVault(v, cfg, globalDec)
		if err != nil {
			return nil, err
		}
		result[v.Name] = dec
	}
	return result, nil
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

	// 3. key_file — supports both raw age key and ward-<base64url> token
	if cfg.Encryption.KeyFile != "" {
		if _, err := os.Stat(cfg.Encryption.KeyFile); err != nil {
			fatalKeyError(
				fmt.Sprintf("key file %s%s%s not found", clrCyan, cfg.Encryption.KeyFile, clrReset),
				fmt.Sprintf("run %sward init%s to generate it, or copy your %s.ward.key%s here", clrBold, clrReset, clrCyan, clrReset),
			)
		}
		data, err := os.ReadFile(cfg.Encryption.KeyFile)
		if err != nil {
			return "", fmt.Errorf("reading key file: %w", err)
		}
		if strings.HasPrefix(strings.TrimSpace(string(data)), "ward-") {
			return writeTempKey(strings.TrimSpace(string(data)))
		}
		return cfg.Encryption.KeyFile, nil
	}

	return "", nil
}

// resolveKeyFileForVault resolves the key for a specific vault.
// Priority: WARD_KEY_<NAME> env var → vault-level key_env → vault-level key_file → falls through to global.
// Returns "" when no vault-specific key is configured (caller should fall back to global).
func resolveKeyFileForVault(v config.Source) (string, error) {
	envName := "WARD_KEY_" + strings.ToUpper(v.Name)
	if token := os.Getenv(envName); token != "" {
		keyFile, err := writeTempKey(token)
		if err != nil {
			return "", fmt.Errorf("decoding %s: %w", envName, err)
		}
		return keyFile, nil
	}
	if v.Encryption.KeyEnv != "" {
		content := os.Getenv(v.Encryption.KeyEnv)
		if content == "" {
			fatalKeyError(
				fmt.Sprintf("env var %s%s%s is empty or not set", clrYellow, v.Encryption.KeyEnv, clrReset),
				fmt.Sprintf("set %s%s%s to the contents of your age key", clrYellow, v.Encryption.KeyEnv, clrReset),
			)
		}
		keyFile, err := writeTempKeyRaw([]byte(content))
		if err != nil {
			return "", fmt.Errorf("writing temp key from %s: %w", v.Encryption.KeyEnv, err)
		}
		return keyFile, nil
	}
	if v.Encryption.KeyFile != "" {
		if _, err := os.Stat(v.Encryption.KeyFile); err != nil {
			fatalKeyError(
				fmt.Sprintf("key file %s%s%s not found", clrCyan, v.Encryption.KeyFile, clrReset),
				fmt.Sprintf("run %sward init%s to generate it, or copy your key here", clrBold, clrReset),
			)
		}
		data, err := os.ReadFile(v.Encryption.KeyFile)
		if err != nil {
			return "", fmt.Errorf("reading key file: %w", err)
		}
		if strings.HasPrefix(strings.TrimSpace(string(data)), "ward-") {
			return writeTempKey(strings.TrimSpace(string(data)))
		}
		return v.Encryption.KeyFile, nil
	}
	return "", nil
}

// decryptorForVault returns a Decryptor for a specific vault, falling back to the
// global decryptor when no vault-specific key is configured.
func decryptorForVault(v config.Source, cfg *config.Config, globalDec sops.Decryptor) (sops.Decryptor, error) {
	keyFile, err := resolveKeyFileForVault(v)
	if err != nil {
		return nil, err
	}
	if keyFile == "" {
		return globalDec, nil
	}
	engine := cfg.Encryption.Engine
	if v.Encryption.Engine != "" {
		engine = v.Encryption.Engine
	}
	switch engine {
	case "sops+age":
		return sops.SopsDecryptor{KeyFile: keyFile}, nil
	case "age+armor", "":
		return wardage.AgeArmorDecryptor{KeyFile: keyFile}, nil
	default:
		return nil, fmt.Errorf("unknown encryption engine %q (supported: age+armor, sops+age)", engine)
	}
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
	clrGrayLight  = "\033[37m"         // white — values
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

	termWidth := terminalWidth()
	// Reserve space for "  ← path:line" at the right side.
	const originReserve = 55
	valueColMax := termWidth - originReserve
	if valueColMax < 20 {
		valueColMax = 20
	}

	maxLen := 0
	for _, l := range lines {
		if l.originFile != "" {
			vl := visibleLen(l.text)
			if vl > maxLen {
				maxLen = vl
			}
		}
	}
	if maxLen > valueColMax {
		maxLen = valueColMax
	}

	for _, l := range lines {
		if l.originFile != "" {
			text := l.text
			vl := visibleLen(text)
			if vl > maxLen {
				text = truncateANSI(text, maxLen)
				vl = maxLen
			}
			pad := maxLen - vl + 4
			if pad < 1 {
				pad = 1
			}
			padding := strings.Repeat(" ", pad)

			// Status color drives arrow, file path, and line number.
			statusClr := clrGreen // active (including conflict winner)
			if l.extra || l.overrides {
				statusClr = clrGray
			} else if l.envConflict {
				statusClr = clrLightRed
			}

			fileClr := clrGrayLight  // white for active/conflict/envConflict
			lineClr := clrMagentaSoft // magenta for active/conflict/envConflict
			if l.extra || l.overrides {
				fileClr = clrGray
				lineClr = clrGray
			}

			var originStr string
			colonStr := clrCyan
			if l.extra || l.overrides {
				colonStr = clrGray
			}
			if l.originLine > 0 {
				originStr = fmt.Sprintf("%s%s%s%s:%s%d%s", fileClr, l.originFile, clrReset, colonStr, lineClr, l.originLine, clrReset)
			} else {
				originStr = fmt.Sprintf("%s%s%s", fileClr, l.originFile, clrReset)
			}
			if l.overrides {
				originStr += fmt.Sprintf(" %s(overridden)%s", clrOrange, clrReset)
			}

			fmt.Printf("%s%s%s←%s %s\n", text, padding, statusClr, clrReset, originStr)
		} else {
			fmt.Println(l.text)
		}
	}

	hasOverrides, hasConflict, hasGhosted, hasEnvConflict := false, false, false, false
	for _, l := range lines {
		if l.overrides {
			hasOverrides = true
		}
		if l.conflict && !l.extra {
			hasConflict = true
		}
		if l.extra {
			hasGhosted = true
		}
		if l.envConflict {
			hasEnvConflict = true
		}
	}

	legend := fmt.Sprintf("\n%s%s●%s active", clrGray, clrGreen, clrGray)
	if hasOverrides {
		legend += fmt.Sprintf("  %s●%s overrides", clrOrange, clrGray)
	}
	if hasConflict || hasEnvConflict {
		legend += fmt.Sprintf("  %s●%s conflict", clrLightRed, clrGray)
	}
	if hasGhosted {
		legend += fmt.Sprintf("  %s●%s ghosted", clrGray, clrGray)
	}
	fmt.Println(legend + clrReset)
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
				text:        fmt.Sprintf("%s%s%s:%s %s%s%s", indentStr, clrGreen, k, clrReset, clrGrayLight, truncateValue(fmt.Sprintf("%v", child.Value), treeValueMaxCols), clrReset),
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
				color = clrGray
			}
			keyColor := color
			colonColor := color
			valueColor := clrGrayLight
			if isOverrides {
				valueColor = clrGray
			}
			*lines = append(*lines, listLine{
				text:        fmt.Sprintf("%s%s%s%s:%s %s%s%s", indentStr, keyColor, k, colonColor, clrReset, valueColor, truncateValue(fmt.Sprintf("%v", child.Value), treeValueMaxCols), clrReset),
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
			text: fmt.Sprintf("%s%s%s%s:", indentStr, clrGrayLight, k, clrReset),
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

const treeValueMaxCols = 120

func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w < 40 {
		w, _, err = term.GetSize(int(os.Stderr.Fd()))
	}
	if err != nil || w < 40 {
		return 80
	}
	return w
}

// truncateValue collapses newlines to spaces and cuts s to max visible chars.
func truncateValue(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimRight(s, " ")
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// truncateANSI truncates s to max visible (non-ANSI) characters, preserving ANSI codes.
func truncateANSI(s string, max int) string {
	var out strings.Builder
	visible := 0
	inEsc := false
	for i := 0; i < len(s); {
		b := s[i]
		if b == '\033' {
			inEsc = true
		}
		if inEsc {
			out.WriteByte(b)
			if b == 'm' {
				inEsc = false
			}
			i++
			continue
		}
		r, size := []rune(s[i:])[0], len(string([]rune(s[i:])[0]))
		if visible >= max {
			out.WriteString("…")
			break
		}
		out.WriteRune(r)
		visible++
		i += size
	}
	return out.String()
}

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
