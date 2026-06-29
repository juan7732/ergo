package github

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecksumFor(t *testing.T) {
	const body = `abc123  ergo-darwin-arm64
def456  ergo-linux-amd64
789aaa *ergo-linux-arm64
`
	dir := t.TempDir()
	path := filepath.Join(dir, "checksums.txt")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	tests := []struct {
		name      string
		asset     string
		want      string
		wantError bool
	}{
		{name: "darwin arm64", asset: "ergo-darwin-arm64", want: "abc123"},
		{name: "linux amd64", asset: "ergo-linux-amd64", want: "def456"},
		{name: "strips binary-mode star", asset: "ergo-linux-arm64", want: "789aaa"},
		{name: "missing asset errors", asset: "ergo-windows-amd64", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ChecksumFor(path, tt.asset)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestChecksumFor_MissingFile(t *testing.T) {
	_, err := ChecksumFor(filepath.Join(t.TempDir(), "nope.txt"), "ergo-darwin-arm64")
	assert.Error(t, err)
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data")
	require.NoError(t, os.WriteFile(path, []byte("ergo-stub\n"), 0o600))

	got, err := FileSHA256(path)
	require.NoError(t, err)
	// printf 'ergo-stub\n' | shasum -a 256
	assert.Equal(t, "326c6425d11878c010951da8bfc91f0d80643423776c9b3673787f1d74261673", got)
}
