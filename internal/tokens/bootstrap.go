package tokens

import (
	"fmt"
	"os"
	"path/filepath"
)

// Bootstrap creates a symlink from tokensPath to Claude Code's
// .credentials.json so the proxy always reads the freshest tokens.
// If tokensPath already exists and force is false, it returns an error.
func Bootstrap(tokensPath string, force bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("determine home dir: %w", err)
	}
	src := filepath.Join(home, ".claude", ".credentials.json")

	// Validate the source file is readable and has valid tokens.
	if _, err := Load(src); err != nil {
		return fmt.Errorf("claude code credentials not usable at %s; run `claude login` first: %w", src, err)
	}

	if !force {
		if _, err := os.Stat(tokensPath); err == nil {
			return fmt.Errorf("tokens file already exists at %s; use --force to overwrite", tokensPath)
		}
	}

	if err := os.MkdirAll(filepath.Dir(tokensPath), 0700); err != nil {
		return fmt.Errorf("create tokens dir: %w", err)
	}

	// Remove any existing file/symlink before creating the new symlink.
	_ = os.Remove(tokensPath)

	if err := os.Symlink(src, tokensPath); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	return nil
}
