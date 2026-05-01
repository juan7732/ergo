package vscode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteIfChanged_WritesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.code-workspace")
	content := []byte(`{"ergo":{"workspace-name":"ws"}}` + "\n")

	written, err := WriteIfChanged(path, content)
	require.NoError(t, err)
	assert.True(t, written)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, content, got)
}

func TestWriteIfChanged_NoOpWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.code-workspace")
	content := []byte(`{"ergo":{"workspace-name":"ws"}}` + "\n")

	require.NoError(t, os.WriteFile(path, content, 0o600))

	written, err := WriteIfChanged(path, content)
	require.NoError(t, err)
	assert.False(t, written, "should be a no-op when content is identical")
}

func TestWriteIfChanged_UpdatesChangedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.code-workspace")
	original := []byte(`{"ergo":{"workspace-name":"old"}}` + "\n")
	updated := []byte(`{"ergo":{"workspace-name":"new"}}` + "\n")

	require.NoError(t, os.WriteFile(path, original, 0o600))

	written, err := WriteIfChanged(path, updated)
	require.NoError(t, err)
	assert.True(t, written)

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, updated, got)
}

func TestWriteIfChanged_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "file.code-workspace")
	content := []byte(`{}` + "\n")

	written, err := WriteIfChanged(path, content)
	require.NoError(t, err)
	assert.True(t, written)

	_, err = os.Stat(path)
	assert.NoError(t, err, "file should exist after write")
}
