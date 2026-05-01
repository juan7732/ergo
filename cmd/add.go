package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"juan7732/ergo/internal/config"
	"juan7732/ergo/internal/tui"
)

var addCmd = &cobra.Command{
	Use:   "add [workspace-name]",
	Short: "Add a repo or folder to a workspace",
	Long: `Add a repo or folder to a workspace definition.

Without subcommands, launches an interactive TUI flow.
Use 'repo' or 'folder' subcommands for non-interactive shorthand.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAdd,
}

var addRepoCmd = &cobra.Command{
	Use:   "repo <url>",
	Short: "Add a repo to the current workspace",
	Long: `Add a repo to a workspace non-interactively.

Examples:
  ergo add repo https://github.com/owner/repo.git
  ergo add repo https://github.com/owner/repo.git --tags=go,tools --group=tools
  ergo add repo https://github.com/owner/utils.git --name=utils-personal`,
	Args: cobra.ExactArgs(1),
	RunE: runAddRepo,
}

var addFolderCmd = &cobra.Command{
	Use:   "folder <name>",
	Short: "Add a folder to the current workspace",
	Long: `Add a non-repo folder to a workspace non-interactively.

Examples:
  ergo add folder scratch
  ergo add folder planning --git`,
	Args: cobra.ExactArgs(1),
	RunE: runAddFolder,
}

func init() {
	addRepoCmd.Flags().String("name", "", "Explicit repo name (overrides derived name)")
	addRepoCmd.Flags().String("tags", "", "Comma-separated tags")
	addRepoCmd.Flags().String("group", "", "Group name")

	addFolderCmd.Flags().Bool("git", false, "Run git init when creating the folder")

	addCmd.AddCommand(addRepoCmd)
	addCmd.AddCommand(addFolderCmd)
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	nameArg := ""
	if len(args) > 0 {
		nameArg = args[0]
	}

	name, err := resolveWorkspaceName(cmd, nameArg)
	if err != nil {
		return err
	}

	globalCfg, err := config.LoadGlobal()
	if err != nil {
		return fmt.Errorf("loading global config: %w", err)
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	form := tui.NewAddForm(globalCfg.Defaults.DefaultBranch, wsCfg.Repos, wsCfg.Folders)
	finalModel, err := tui.RunInline(form)
	if err != nil {
		return fmt.Errorf("running add form: %w", err)
	}

	result, confirmed := finalModel.(tui.AddForm).Result()
	if !confirmed {
		fmt.Fprintln(cmd.OutOrStdout(), "cancelled")
		return nil
	}

	if result.IsRepo {
		wsCfg.Repos = append(wsCfg.Repos, result.Repo)
		if err := config.WriteWorkspace(name, wsCfg); err != nil {
			return fmt.Errorf("writing workspace config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "added repo %q to workspace %q\n", result.Repo.EffectiveName(), name)
	} else {
		wsCfg.Folders = append(wsCfg.Folders, result.Folder)
		if err := config.WriteWorkspace(name, wsCfg); err != nil {
			return fmt.Errorf("writing workspace config: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "added folder %q to workspace %q\n", result.Folder.Name, name)
	}

	return promptSync(cmd, name)
}

func runAddRepo(cmd *cobra.Command, args []string) error {
	url := args[0]
	explicitName, _ := cmd.Flags().GetString("name")
	tagsStr, _ := cmd.Flags().GetString("tags")
	group, _ := cmd.Flags().GetString("group")

	name, err := resolveWorkspaceName(cmd, "")
	if err != nil {
		return err
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	effectiveName := explicitName
	if effectiveName == "" {
		effectiveName = config.DeriveRepoName(url)
	}
	if collErr := checkNameCollision(effectiveName, wsCfg); collErr != nil {
		if explicitName == "" {
			return fmt.Errorf("%w; use --name to provide a different name", collErr)
		}
		return collErr
	}

	repo := config.Repo{URL: url, Group: group}
	if explicitName != "" {
		repo.Name = &explicitName
	}
	if tagsStr != "" {
		repo.Tags = splitCSV(tagsStr)
	}

	wsCfg.Repos = append(wsCfg.Repos, repo)
	if err := config.WriteWorkspace(name, wsCfg); err != nil {
		return fmt.Errorf("writing workspace config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "added repo %q to workspace %q\n", effectiveName, name)
	return promptSync(cmd, name)
}

func runAddFolder(cmd *cobra.Command, args []string) error {
	folderName := args[0]
	gitInit, _ := cmd.Flags().GetBool("git")

	name, err := resolveWorkspaceName(cmd, "")
	if err != nil {
		return err
	}

	wsCfg, err := config.LoadWorkspace(name)
	if err != nil {
		return fmt.Errorf("loading workspace config: %w", err)
	}

	if err := checkNameCollision(folderName, wsCfg); err != nil {
		return err
	}

	folder := config.Folder{Name: folderName, Git: gitInit}
	wsCfg.Folders = append(wsCfg.Folders, folder)
	if err := config.WriteWorkspace(name, wsCfg); err != nil {
		return fmt.Errorf("writing workspace config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "added folder %q to workspace %q\n", folderName, name)
	return promptSync(cmd, name)
}

// checkNameCollision returns an error if the given name already exists as a
// repo or folder in the workspace config.
func checkNameCollision(name string, cfg config.WorkspaceConfig) error {
	for _, r := range cfg.Repos {
		if r.EffectiveName() == name {
			return fmt.Errorf("name %q already exists as a repo in this workspace", name)
		}
	}
	for _, f := range cfg.Folders {
		if f.Name == name {
			return fmt.Errorf("name %q already exists as a folder in this workspace", name)
		}
	}
	return nil
}

// promptSync asks whether to sync the workspace now and does so if confirmed.
// Only prompts when stdin is a terminal; silently skips otherwise.
func promptSync(cmd *cobra.Command, name string) error {
	if !isTerminal() {
		return nil
	}
	out := cmd.OutOrStdout()
	fmt.Fprint(out, "sync workspace now? [y/N] ")
	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil
	}
	if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
		return nil
	}
	return runSync(cmd, []string{name})
}

// splitCSV splits a comma-separated string and trims whitespace from each part.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
