package cmd

import (
	"fmt"
	"os"
)

// currentDir returns the current working directory.
func currentDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}
	return cwd, nil
}
