package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func NewGetCmd() *cobra.Command {
	var anchorPath string

	c := &cobra.Command{
		Use:               "get [dot.path]",
		Short:             "Return the merged value at a dot-path",
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeDotPaths,
		Run: func(_ *cobra.Command, args []string) {
			eng, err := newEngine()
			if err != nil {
				fatal(err)
			}
			result, err := eng.Merge(anchorPath)
			if err != nil {
				fatal(err)
			}

			dotPath := ""
			if len(args) == 1 {
				dotPath = args[0]
			} else {
				// Interactive picker
				all := collectDotPaths(result.Tree, "")
				sort.Strings(all)
				dotPath = pickDotPath(all)
				if dotPath == "" {
					return
				}
			}

			node, err := eng.GetAtPath(result, dotPath)
			if err != nil {
				fatal(err)
			}
			printTree(node, 0)
		},
	}

	c.Flags().StringVarP(&anchorPath, "anchor", "a", "", "anchor .ward file to scope the merge")
	return c
}

// pickDotPath shows an interactive filtered list and returns the chosen path.
func pickDotPath(paths []string) string {
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "ward: no keys found")
		return ""
	}

	filter := ""
	cursor := 0
	visible := paths

	fmt.Fprintf(os.Stderr, "\n  %sward get%s — type to filter, %s↑↓%s navigate, %sENTER%s select, %sESC%s cancel\n\n",
		clrBold, clrReset, clrCyan, clrReset, clrGreen, clrReset, clrGray, clrReset)

	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback: line-based input
		return pickDotPathLine(paths)
	}
	defer term.Restore(fd, oldState)

	redraw := func() {
		// Recompute visible
		if filter != "" {
			visible = nil
			for _, p := range paths {
				if strings.Contains(p, filter) {
					visible = append(visible, p)
				}
			}
		} else {
			visible = paths
		}
		if cursor >= len(visible) {
			cursor = max(0, len(visible)-1)
		}

		limit := 12
		if len(visible) < limit {
			limit = len(visible)
		}

		// Print filter line + list (use \r\n — raw mode doesn't auto-CR on \n)
		fmt.Fprintf(os.Stderr, "\r  %s>%s %s\r\n", clrCyan, clrReset, filter)
		for i := 0; i < limit; i++ {
			p := visible[i]
			highlighted := p
			if filter != "" {
				highlighted = strings.ReplaceAll(p, filter, clrGreen+filter+clrReset)
			}
			if i == cursor {
				fmt.Fprintf(os.Stderr, "  %s>%s %s\r\n", clrCyan, clrReset, highlighted)
			} else {
				fmt.Fprintf(os.Stderr, "    %s\r\n", highlighted)
			}
		}
		if len(visible) > 12 {
			fmt.Fprintf(os.Stderr, "  %s… %d more%s\r\n", clrGray, len(visible)-12, clrReset)
		}
		if len(visible) == 0 {
			fmt.Fprintf(os.Stderr, "  %sno matches%s\r\n", clrGray, clrReset)
		}
	}

	clearDisplay := func() {
		limit := 12
		if len(visible) < limit {
			limit = len(visible)
		}
		lines := limit + 2 // filter line + list
		if len(visible) > 12 || len(visible) == 0 {
			lines++
		}
		clearLines(lines)
	}

	redraw()

	buf := make([]byte, 4)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			clearDisplay()
			return ""
		}

		b := buf[:n]
		switch {
		case n == 1 && b[0] == 27: // ESC
			clearDisplay()
			return ""
		case n == 1 && (b[0] == 13 || b[0] == 10): // ENTER
			clearDisplay()
			if len(visible) == 0 {
				return ""
			}
			if cursor < len(visible) {
				return visible[cursor]
			}
			return visible[0]
		case n == 1 && (b[0] == 127 || b[0] == 8): // backspace
			if len(filter) > 0 {
				filter = filter[:len(filter)-1]
				cursor = 0
			}
		case n == 3 && b[0] == 27 && b[1] == '[' && b[2] == 'A': // up arrow
			if cursor > 0 {
				cursor--
			}
		case n == 3 && b[0] == 27 && b[1] == '[' && b[2] == 'B': // down arrow
			limit := 12
			if len(visible) < limit {
				limit = len(visible)
			}
			if cursor < limit-1 {
				cursor++
			}
		default:
			if n == 1 && b[0] >= 32 && b[0] < 127 {
				filter += string(rune(b[0]))
				cursor = 0
			}
		}
		clearDisplay()
		redraw()
	}
}

// pickDotPathLine is a simple line-based fallback when raw mode is unavailable.
func pickDotPathLine(paths []string) string {
	fmt.Fprintf(os.Stderr, "  %s> %s", clrCyan, clrReset)
	var filter string
	fmt.Fscan(os.Stdin, &filter)
	for _, p := range paths {
		if p == filter {
			return p
		}
	}
	for _, p := range paths {
		if strings.Contains(p, filter) {
			return p
		}
	}
	return ""
}

func clearLines(n int) {
	for i := 0; i < n; i++ {
		fmt.Fprint(os.Stderr, "\033[A\033[2K")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
