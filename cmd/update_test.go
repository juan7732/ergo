package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReleaseAssetName(t *testing.T) {
	tests := []struct {
		goos   string
		goarch string
		want   string
	}{
		{"darwin", "arm64", "ergo-darwin-arm64"},
		{"darwin", "amd64", "ergo-darwin-amd64"},
		{"linux", "amd64", "ergo-linux-amd64"},
		{"linux", "arm64", "ergo-linux-arm64"},
		{"windows", "amd64", "ergo-windows-amd64.exe"},
		{"windows", "arm64", "ergo-windows-arm64.exe"},
	}
	for _, tt := range tests {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			assert.Equal(t, tt.want, releaseAssetName(tt.goos, tt.goarch))
		})
	}
}
