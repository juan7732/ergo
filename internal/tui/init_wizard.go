package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"juan7732/ergo/internal/config"
)

type wizardStep int

const (
	stepWorkspaceName wizardStep = iota
	stepRepoURL
	stepRepoName
	stepRepoBranch
	stepRepoTags
	stepRepoGroup
	stepFolderName
	stepFolderGit
	stepConfirm
	stepDone
)

// WizardResult holds the data collected by the init wizard.
type WizardResult struct {
	Name    string
	Repos   []config.Repo
	Folders []config.Folder
}

// InitWizard is a Bubble Tea model for the ergo init guided flow.
type InitWizard struct {
	step          wizardStep
	input         textinput.Model
	defaultBranch string

	result WizardResult

	// Partially-built entries during the repo/folder loops.
	pendingRepo   config.Repo
	pendingFolder config.Folder

	confirmed bool
	cancelled bool
}

// NewInitWizard creates an InitWizard ready to run.
// prefillName is the workspace name from the CLI arg (may be empty).
func NewInitWizard(prefillName, defaultBranch string) InitWizard {
	ti := newInput(prefillName, "my-workspace", 64)
	return InitWizard{
		step:          stepWorkspaceName,
		input:         ti,
		defaultBranch: defaultBranch,
	}
}

func (m InitWizard) Init() tea.Cmd {
	return textinput.Blink
}

func (m InitWizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEnter:
			return m.handleEnter()
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m InitWizard) handleEnter() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.input.Value())

	switch m.step {

	case stepWorkspaceName:
		if val == "" {
			return m, nil // name is required
		}
		m.result.Name = val
		m.step = stepRepoURL
		m.input = newInput("", "https://github.com/owner/repo.git", 256)

	case stepRepoURL:
		if val == "" {
			// Repo entry done; proceed to folders.
			m.step = stepFolderName
			m.input = newInput("", "scratch", 64)
		} else {
			m.pendingRepo = config.Repo{URL: val}
			derived := config.DeriveRepoName(val)
			m.step = stepRepoName
			m.input = newInput(derived, derived, 64)
		}

	case stepRepoName:
		name := val
		if name == "" {
			name = config.DeriveRepoName(m.pendingRepo.URL)
		}
		m.pendingRepo.Name = &name
		m.step = stepRepoBranch
		m.input = newInput(m.defaultBranch, m.defaultBranch, 64)

	case stepRepoBranch:
		branch := val
		if branch == "" {
			branch = m.defaultBranch
		}
		m.pendingRepo.Branch = &branch
		m.step = stepRepoTags
		m.input = newInput("", "ml, python", 128)

	case stepRepoTags:
		if val != "" {
			m.pendingRepo.Tags = splitTrimmed(val)
		}
		m.step = stepRepoGroup
		m.input = newInput("", "ml", 64)

	case stepRepoGroup:
		m.pendingRepo.Group = val
		m.result.Repos = append(m.result.Repos, m.pendingRepo)
		m.pendingRepo = config.Repo{}
		m.step = stepRepoURL
		m.input = newInput("", "https://github.com/owner/repo.git", 256)

	case stepFolderName:
		if val == "" {
			// Folder entry done; proceed to confirm.
			m.step = stepConfirm
		} else {
			m.pendingFolder = config.Folder{Name: val}
			m.step = stepFolderGit
			m.input = newInput("", "y/N", 4)
		}

	case stepFolderGit:
		m.pendingFolder.Git = strings.ToLower(val) == "y"
		m.result.Folders = append(m.result.Folders, m.pendingFolder)
		m.pendingFolder = config.Folder{}
		m.step = stepFolderName
		m.input = newInput("", "planning", 64)

	case stepConfirm:
		m.confirmed = true
		m.step = stepDone
		return m, tea.Quit
	}

	return m, textinput.Blink
}

func (m InitWizard) View() string {
	if m.cancelled || m.step == stepDone {
		return ""
	}

	var b strings.Builder
	b.WriteString(StyleTitle.Render("ergo init") + "\n\n")

	// Accumulated repos.
	if len(m.result.Repos) > 0 {
		b.WriteString(StyleSubtle.Render("Repos added:") + "\n")
		for _, r := range m.result.Repos {
			b.WriteString("  " + StyleSuccess.Render("✓") + " " + r.EffectiveName() + " " + StyleSubtle.Render(r.URL) + "\n")
		}
		b.WriteString("\n")
	}

	// Accumulated folders.
	if len(m.result.Folders) > 0 {
		b.WriteString(StyleSubtle.Render("Folders added:") + "\n")
		for _, f := range m.result.Folders {
			gitHint := ""
			if f.Git {
				gitHint = StyleSubtle.Render(" (git init)")
			}
			b.WriteString("  " + StyleSuccess.Render("✓") + " " + f.Name + gitHint + "\n")
		}
		b.WriteString("\n")
	}

	switch m.step {
	case stepWorkspaceName:
		b.WriteString(StyleLabel.Render("Workspace name") + "\n")
		b.WriteString(m.input.View() + "\n")

	case stepRepoURL:
		b.WriteString(StyleLabel.Render("Repo URL") + "  " + StyleSubtle.Render("blank to finish") + "\n")
		b.WriteString(m.input.View() + "\n")

	case stepRepoName:
		b.WriteString(StyleLabel.Render("Repo name") + "  " + StyleSubtle.Render("blank = use derived name") + "\n")
		b.WriteString(m.input.View() + "\n")

	case stepRepoBranch:
		b.WriteString(StyleLabel.Render("Branch") + "\n")
		b.WriteString(m.input.View() + "\n")

	case stepRepoTags:
		b.WriteString(StyleLabel.Render("Tags") + "  " + StyleSubtle.Render("comma-separated, optional") + "\n")
		b.WriteString(m.input.View() + "\n")

	case stepRepoGroup:
		b.WriteString(StyleLabel.Render("Group") + "  " + StyleSubtle.Render("optional") + "\n")
		b.WriteString(m.input.View() + "\n")

	case stepFolderName:
		b.WriteString(StyleLabel.Render("Folder name") + "  " + StyleSubtle.Render("blank to finish") + "\n")
		b.WriteString(m.input.View() + "\n")

	case stepFolderGit:
		b.WriteString(StyleLabel.Render(fmt.Sprintf("Run git init in %q?", m.pendingFolder.Name)) + "\n")
		b.WriteString(m.input.View() + "  " + StyleSubtle.Render("y/N") + "\n")

	case stepConfirm:
		b.WriteString(StyleLabel.Render("Summary") + "\n\n")
		b.WriteString("  Workspace: " + StyleSelected.Render(m.result.Name) + "\n")
		b.WriteString(fmt.Sprintf("  Repos:     %d\n", len(m.result.Repos)))
		b.WriteString(fmt.Sprintf("  Folders:   %d\n", len(m.result.Folders)))
		b.WriteString("\n")
		b.WriteString(KeybindingHint("enter", "confirm") + "  " + KeybindingHint("esc", "cancel") + "\n")
		return b.String()
	}

	b.WriteString("\n" + KeybindingHint("enter", "next") + "  " + KeybindingHint("esc", "cancel") + "\n")
	return b.String()
}

// Result returns the collected wizard data and whether the user confirmed.
// Only meaningful after the Bubble Tea program exits.
func (m InitWizard) Result() (WizardResult, bool) {
	return m.result, m.confirmed
}

// newInput creates a focused textinput with a pre-filled value and placeholder.
func newInput(value, placeholder string, charLimit int) textinput.Model {
	ti := textinput.New()
	ti.Focus()
	ti.SetValue(value)
	ti.Placeholder = placeholder
	ti.CharLimit = charLimit
	// Position cursor at end of pre-filled value.
	ti.CursorEnd()
	return ti
}

// splitTrimmed splits a comma-separated string and trims whitespace from each part.
func splitTrimmed(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
