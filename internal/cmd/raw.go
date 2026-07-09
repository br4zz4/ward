package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/br4zz4/ward/internal/secrets"
	"github.com/br4zz4/ward/internal/ward"
	"github.com/spf13/cobra"
)

func NewRawCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "raw [file.ward]",
		Short: "Print the decrypted YAML of a .ward file (all files when none given)",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}

			if len(args) == 1 {
				rawOne(eng, args[0])
				return
			}
			rawAll(eng)
		},
	}
}

// rawOne prints just the decrypted YAML of a single file (no header), matching
// the original behaviour for piping.
func rawOne(eng *ward.Engine, path string) {
	if err := requireWardFile(path); err != nil {
		fatal(err)
	}
	plain, err := eng.Decrypt(path)
	if err != nil {
		fatal(fmt.Errorf("decrypting %s: %w", path, err))
	}
	os.Stdout.Write(plain)
}

// rawAll prints every discovered .ward file with a git-delta-style header naming
// the file, followed by its decrypted content.
func rawAll(eng *ward.Engine) {
	files, err := secrets.Discover(eng.SourcePaths())
	if err != nil {
		fatal(fmt.Errorf("discovering files: %w", err))
	}
	if len(files) == 0 {
		fatalNoSources()
	}

	for i, path := range files {
		if i > 0 {
			fmt.Println()
		}
		fmt.Print(rawHeader(path))
		plain, err := eng.Decrypt(path)
		if err != nil {
			fatal(fmt.Errorf("decrypting %s: %w", path, err))
		}
		fmt.Print(indent(plain))
	}
}

// rawHeader renders a file header in the style of git-delta: the path in bold
// cyan on its own line, underlined by a rule the width of the path.
func rawHeader(path string) string {
	rule := strings.Repeat("─", utf8.RuneCountInString(path))
	return fmt.Sprintf("%s%s%s\n%s%s%s\n", clrBold+clrCyan, path, clrReset, clrGray, rule, clrReset)
}

// indent returns content with each line prefixed by two spaces so it reads as
// nested under its header, with the trailing blank line dropped.
func indent(content []byte) string {
	trimmed := bytes.TrimRight(content, "\n")
	if len(trimmed) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, line := range bytes.Split(trimmed, []byte("\n")) {
		if len(line) == 0 {
			sb.WriteByte('\n')
			continue
		}
		sb.WriteString("  ")
		sb.Write(line)
		sb.WriteByte('\n')
	}
	return sb.String()
}
