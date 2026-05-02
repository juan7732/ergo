//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"juan7732/ergo/test/integration/harness"
)

// TestVersion_PrintsLDFlagValue ensures the binary was built with the expected
// version ldflag (set in the Dockerfile to "integration").
func TestVersion_PrintsLDFlagValue(t *testing.T) {
	t.Parallel()
	h := harness.New(t)

	res := h.Run("--version")
	res.AssertOK(t)

	assert.True(t,
		strings.Contains(res.Stdout, "integration") || strings.Contains(res.Combined, "integration"),
		"expected version output to contain ldflag value 'integration'; got:\n%s", res.Combined,
	)
}
