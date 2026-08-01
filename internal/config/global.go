package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// ergoHome returns the path to the ergo configuration directory (~/.ergo).
func ergoHome() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".ergo"), nil
}

// defaultGlobalConfig returns the default GlobalConfig values used when
// no config file exists yet.
func defaultGlobalConfig() GlobalConfig {
	return GlobalConfig{
		Defaults: DefaultsConfig{
			WorkspaceRoot: "~/ergo-workspaces",
			DefaultBranch: "main",
		},
		Parallel: ParallelConfig{
			Enabled:   true,
			BatchSize: 4,
		},
		Sync: SyncConfig{
			AutoPull: true,
		},
		Run: RunConfig{
			ExcludedGroups: []string{},
		},
		Git: GitConfig{
			Protocol: GitProtocolHTTPS,
		},
	}
}

// LoadGlobal loads ~/.ergo/config.toml. If the file does not exist, it is
// created with default values and the defaults are returned.
func LoadGlobal() (GlobalConfig, error) {
	home, err := ergoHome()
	if err != nil {
		return GlobalConfig{}, err
	}

	path := filepath.Join(home, "config.toml")
	_, statErr := os.Stat(path)
	if errors.Is(statErr, os.ErrNotExist) {
		cfg := defaultGlobalConfig()
		if err := writeGlobalConfig(path, cfg); err != nil {
			return GlobalConfig{}, fmt.Errorf("creating default global config: %w", err)
		}
		return cfg, nil
	}
	if statErr != nil {
		return GlobalConfig{}, fmt.Errorf("checking global config: %w", statErr)
	}

	var cfg GlobalConfig
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return GlobalConfig{}, fmt.Errorf("parsing global config %s: %w", path, err)
	}
	if p := strings.TrimSpace(cfg.Git.Protocol); p != "" &&
		!strings.EqualFold(p, GitProtocolHTTPS) && !strings.EqualFold(p, GitProtocolSSH) {
		return GlobalConfig{}, fmt.Errorf("invalid [git] protocol %q in %s: must be %q or %q",
			p, path, GitProtocolSSH, GitProtocolHTTPS)
	}
	return cfg, nil
}

// writeGlobalConfig serializes cfg as TOML and writes it to path, creating
// any necessary parent directories.
func writeGlobalConfig(path string, cfg GlobalConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating ergo home directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("opening global config for write: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encoding global config: %w", err)
	}
	return nil
}

// ErgoHome is the exported accessor used by other packages that need the
// ergo home directory without importing it from config directly.
func ErgoHome() (string, error) {
	return ergoHome()
}
