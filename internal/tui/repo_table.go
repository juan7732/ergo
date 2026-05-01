package tui

import (
	"fmt"
	"strings"

	"juan7732/ergo/internal/workspace"
)

// RenderRepoTable renders a bordered table of repo statuses using Unicode
// box-drawing characters. Returns a complete string ready to print.
func RenderRepoTable(statuses []workspace.RepoStatusEntry) string {
	// Compute column widths from content, lower-bounded by header widths.
	wName := len("Repo")
	wBranch := len("Branch")
	wStatus := len("uncloned") // widest possible status value
	wBehind := len("Behind")
	wGroup := len("Group")

	for _, s := range statuses {
		if n := len(s.Name); n > wName {
			wName = n
		}
		if n := len(s.Branch); n > wBranch {
			wBranch = n
		}
		if n := len(s.Group); n > wGroup {
			wGroup = n
		}
	}

	// borderLine returns a horizontal separator using the given edge and junction runes.
	borderLine := func(left, mid, right, fill string) string {
		return left +
			strings.Repeat(fill, wName+2) + mid +
			strings.Repeat(fill, wBranch+2) + mid +
			strings.Repeat(fill, wStatus+2) + mid +
			strings.Repeat(fill, wBehind+2) + mid +
			strings.Repeat(fill, wGroup+2) + right
	}

	// dataRow formats a single row with left/right pipe borders.
	dataRow := func(name, branch, status, behind, group string) string {
		return fmt.Sprintf("│ %-*s │ %-*s │ %-*s │ %-*s │ %-*s │",
			wName, name,
			wBranch, branch,
			wStatus, status,
			wBehind, behind,
			wGroup, group,
		)
	}

	var sb strings.Builder

	sb.WriteString(StyleTableBorder.Render(borderLine("┌", "┬", "┐", "─")))
	sb.WriteString("\n")
	sb.WriteString(StyleTableBorder.Render("│") +
		StyleTableHeader.Render(fmt.Sprintf(" %-*s ", wName, "Repo")) +
		StyleTableBorder.Render("│") +
		StyleTableHeader.Render(fmt.Sprintf(" %-*s ", wBranch, "Branch")) +
		StyleTableBorder.Render("│") +
		StyleTableHeader.Render(fmt.Sprintf(" %-*s ", wStatus, "Status")) +
		StyleTableBorder.Render("│") +
		StyleTableHeader.Render(fmt.Sprintf(" %-*s ", wBehind, "Behind")) +
		StyleTableBorder.Render("│") +
		StyleTableHeader.Render(fmt.Sprintf(" %-*s ", wGroup, "Group")) +
		StyleTableBorder.Render("│"))
	sb.WriteString("\n")
	sb.WriteString(StyleTableBorder.Render(borderLine("├", "┼", "┤", "─")))
	sb.WriteString("\n")

	for _, s := range statuses {
		status, behind, branch := statusValues(s)
		sb.WriteString(dataRow(s.Name, branch, status, behind, s.Group))
		sb.WriteString("\n")
	}

	sb.WriteString(StyleTableBorder.Render(borderLine("└", "┴", "┘", "─")))
	sb.WriteString("\n")

	return sb.String()
}

// ShortRepoLine formats a single repo status as a tab-separated line for
// scriptable output (--short flag).
func ShortRepoLine(s workspace.RepoStatusEntry) string {
	status, behind, branch := statusValues(s)
	group := s.Group
	if group == "" {
		group = "-"
	}
	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s", s.Name, branch, status, behind, group)
}

// statusValues derives display strings for status, behind, and branch from a RepoStatusEntry.
func statusValues(s workspace.RepoStatusEntry) (status, behind, branch string) {
	branch = s.Branch
	status = "clean"
	behind = "—"

	if s.Uncloned {
		status = "uncloned"
		branch = "—"
		return
	}
	if s.Dirty {
		status = "dirty"
	}
	if s.Behind > 0 {
		behind = fmt.Sprintf("%d", s.Behind)
	}
	return
}
