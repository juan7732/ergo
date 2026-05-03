package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/juan7732/ergo/internal/config"
)

type addFormStep int

const (
	addStepKind addFormStep = iota
	addStepRepoURL
	addStepRepoName
	addStepRepoBranch
	addStepRepoTags
	addStepRepoGroup
	addStepFolderName
	addStepFolderGit
	addStepDone
)

// AddFormResult holds the outcome of the add form.
// Exactly one of Repo or Folder is populated based on IsRepo.
type AddFormResult struct {
	IsRepo bool
	Repo   config.Repo
	Folder config.Folder
}

// AddForm is a Bubble Tea model for adding a single repo or folder to a workspace.
type AddForm struct {
	step          addFormStep
	input         textinput.Model
	defaultBranch string

	existingRepoNames   map[string]struct{}
	existingFolderNames map[string]struct{}

	result    AddFormResult
	collision string

	confirmed bool
	cancelled bool
}

// NewAddForm creates an AddForm ready to run.
// Existing repos and folders are provided so collision warnings can be shown inline.
func NewAddForm(defaultBranch string, existingRepos []config.Repo, existingFolders []config.Folder) AddForm {
	repoNames := make(map[string]struct{}, len(existingRepos))
	for _, r := range existingRepos {
		repoNames[r.EffectiveName()] = struct{}{}
	}
	folderNames := make(map[string]struct{}, len(existingFolders))
	for _, f := range existingFolders {
		folderNames[f.Name] = struct{}{}
	}
	return AddForm{
		step:                addStepKind,
		input:               newInput("", "r/f", 1),
		defaultBranch:       defaultBranch,
		existingRepoNames:   repoNames,
		existingFolderNames: folderNames,
	}
}

func (m AddForm) Init() tea.Cmd {
	return textinput.Blink
}

func (m AddForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	// Clear collision warning as the user types a new value.
	if m.collision != "" {
		m.collision = ""
	}
	return m, cmd
}

func (m AddForm) handleEnter() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(m.input.Value())

	switch m.step {

	case addStepKind:
		switch strings.ToLower(val) {
		case "r", "repo":
			m.result.IsRepo = true
			m.step = addStepRepoURL
			m.input = newInput("", "https://github.com/owner/repo.git", 256)
		case "f", "folder":
			m.result.IsRepo = false
			m.step = addStepFolderName
			m.input = newInput("", "scratch", 64)
		default:
			return m, nil // invalid; stay on this step
		}

	case addStepRepoURL:
		if val == "" {
			return m, nil // URL is required
		}
		m.result.Repo.URL = val
		derived := config.DeriveRepoName(val)
		m.step = addStepRepoName
		m.input = newInput(derived, derived, 64)

	case addStepRepoName:
		name := val
		if name == "" {
			name = config.DeriveRepoName(m.result.Repo.URL)
		}
		if _, exists := m.existingRepoNames[name]; exists {
			m.collision = fmt.Sprintf("name %q already exists as a repo; pick a different name", name)
			m.input = newInput("", "pick-a-unique-name", 64)
			return m, textinput.Blink
		}
		if _, exists := m.existingFolderNames[name]; exists {
			m.collision = fmt.Sprintf("name %q collides with an existing folder; pick a different name", name)
			m.input = newInput("", "pick-a-unique-name", 64)
			return m, textinput.Blink
		}
		m.result.Repo.Name = &name
		m.step = addStepRepoBranch
		m.input = newInput(m.defaultBranch, m.defaultBranch, 64)

	case addStepRepoBranch:
		branch := val
		if branch == "" {
			branch = m.defaultBranch
		}
		m.result.Repo.Branch = &branch
		m.step = addStepRepoTags
		m.input = newInput("", "ml, python", 128)

	case addStepRepoTags:
		if val != "" {
			m.result.Repo.Tags = splitTrimmed(val)
		}
		m.step = addStepRepoGroup
		m.input = newInput("", "ml", 64)

	case addStepRepoGroup:
		m.result.Repo.Group = val
		m.confirmed = true
		m.step = addStepDone
		return m, tea.Quit

	case addStepFolderName:
		if val == "" {
			return m, nil // name is required
		}
		if _, exists := m.existingRepoNames[val]; exists {
			m.collision = fmt.Sprintf("name %q collides with an existing repo; pick a different name", val)
			m.input = newInput("", "pick-a-unique-name", 64)
			return m, textinput.Blink
		}
		if _, exists := m.existingFolderNames[val]; exists {
			m.collision = fmt.Sprintf("folder %q already exists; pick a different name", val)
			m.input = newInput("", "pick-a-unique-name", 64)
			return m, textinput.Blink
		}
		m.result.Folder.Name = val
		m.step = addStepFolderGit
		m.input = newInput("", "y/N", 4)

	case addStepFolderGit:
		m.result.Folder.Git = strings.ToLower(val) == "y"
		m.confirmed = true
		m.step = addStepDone
		return m, tea.Quit
	}

	return m, textinput.Blink
}

func (m AddForm) View() string {
	if m.cancelled || m.step == addStepDone {
		return ""
	}

	var b strings.Builder
	b.WriteString(StyleTitle.Render("ergo add") + "\n\n")

	if m.collision != "" {
		b.WriteString(StyleError.Render("⚠  "+m.collision) + "\n\n")
	}

	switch m.step {
	case addStepKind:
		b.WriteString(StyleLabel.Render("Add a") + "  " + StyleSubtle.Render("r=repo  f=folder") + "\n")
		b.WriteString(m.input.View() + "\n")

	case addStepRepoURL:
		b.WriteString(StyleLabel.Render("Repo URL") + "\n")
		b.WriteString(m.input.View() + "\n")

	case addStepRepoName:
		b.WriteString(StyleLabel.Render("Repo name") + "  " + StyleSubtle.Render("blank = use derived name") + "\n")
		b.WriteString(m.input.View() + "\n")

	case addStepRepoBranch:
		b.WriteString(StyleLabel.Render("Branch") + "\n")
		b.WriteString(m.input.View() + "\n")

	case addStepRepoTags:
		b.WriteString(StyleLabel.Render("Tags") + "  " + StyleSubtle.Render("comma-separated, optional") + "\n")
		b.WriteString(m.input.View() + "\n")

	case addStepRepoGroup:
		b.WriteString(StyleLabel.Render("Group") + "  " + StyleSubtle.Render("optional") + "\n")
		b.WriteString(m.input.View() + "\n")

	case addStepFolderName:
		b.WriteString(StyleLabel.Render("Folder name") + "\n")
		b.WriteString(m.input.View() + "\n")

	case addStepFolderGit:
		b.WriteString(StyleLabel.Render(fmt.Sprintf("Run git init in %q?", m.result.Folder.Name)) + "  " + StyleSubtle.Render("y/N") + "\n")
		b.WriteString(m.input.View() + "\n")
	}

	b.WriteString("\n" + KeybindingHint("enter", "next") + "  " + KeybindingHint("esc", "cancel") + "\n")
	return b.String()
}

// Result returns the collected form data and whether the user confirmed.
// Only meaningful after the Bubble Tea program exits.
func (m AddForm) Result() (AddFormResult, bool) {
	return m.result, m.confirmed
}
