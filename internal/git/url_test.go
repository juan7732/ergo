package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRewriteToSSH covers https→scp rewriting and every pass-through case.
func TestRewriteToSSH(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"https with .git", "https://github.com/o/r.git", "git@github.com:o/r.git"},
		{"https without .git", "https://github.com/o/r", "git@github.com:o/r"},
		{"trailing slash stripped", "https://github.com/o/r.git/", "git@github.com:o/r.git"},
		{"nested gitlab groups", "https://gitlab.com/group/sub/proj.git", "git@gitlab.com:group/sub/proj.git"},
		{"bitbucket", "https://bitbucket.org/o/r.git", "git@bitbucket.org:o/r.git"},
		{"self-hosted", "https://git.corp.example/team/repo.git", "git@git.corp.example:team/repo.git"},
		{"plain http", "http://git.internal/o/r.git", "git@git.internal:o/r.git"},
		{"surrounding whitespace", "  https://github.com/o/r.git  ", "git@github.com:o/r.git"},
		{"explicit port unchanged", "https://git.corp.com:8443/o/r.git", "https://git.corp.com:8443/o/r.git"},
		{"embedded credentials unchanged", "https://user:tok@github.com/o/r.git", "https://user:tok@github.com/o/r.git"},
		{"already scp unchanged", "git@github.com:o/r.git", "git@github.com:o/r.git"},
		{"ssh scheme unchanged", "ssh://git@github.com/o/r.git", "ssh://git@github.com/o/r.git"},
		{"git scheme unchanged", "git://github.com/o/r.git", "git://github.com/o/r.git"},
		{"file scheme unchanged", "file:///srv/git/r.git", "file:///srv/git/r.git"},
		{"absolute path unchanged", "/srv/git/r.git", "/srv/git/r.git"},
		{"relative path unchanged", "../r", "../r"},
		{"host only unchanged", "https://github.com/", "https://github.com/"},
		{"empty unchanged", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, RewriteToSSH(tc.in))
		})
	}
}
