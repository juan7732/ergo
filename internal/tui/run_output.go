package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/juan7732/ergo/internal/workspace"
)

// PrintRunResult writes the formatted output of a single run result to w.
//
// Format:
//
//	━━━ <name> ━━━
//	<output>
//
// A blank line is always appended to visually separate consecutive results.
func PrintRunResult(w io.Writer, result workspace.RunResult) {
	fmt.Fprintln(w, StyleTitle.Render("━━━ "+result.Name+" ━━━"))

	if result.Err != nil {
		fmt.Fprintln(w, StyleError.Render("error: "+result.Err.Error()))
	} else {
		output := strings.TrimRight(result.Output, "\n")
		if output != "" {
			fmt.Fprintln(w, output)
		}
		if result.ExitCode != 0 {
			fmt.Fprintln(w, StyleError.Render(fmt.Sprintf("exit code %d", result.ExitCode)))
		}
	}

	fmt.Fprintln(w)
}
