package git

import (
	"net/url"
	"strings"
)

// RewriteToSSH converts an https:// or http:// git URL to scp-style SSH form
// (git@host:owner/repo.git). URLs that are not plain-host http(s) URLs are
// returned unchanged: already-SSH forms, ssh://, git://, file://, local paths,
// URLs with embedded credentials, and URLs with explicit ports.
//
// DECISION: URLs with an explicit port pass through unchanged — an https port
// is not the sshd port, and scp form cannot express one; guessing would break
// self-hosted setups. Same for embedded credentials: the user explicitly opted
// into token auth there. Exotic SSH users/hosts belong in the workspace TOML
// as scp URLs directly.
func RewriteToSSH(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") {
		return rawURL
	}

	u, err := url.Parse(trimmed)
	if err != nil || u.User != nil || u.Port() != "" {
		return rawURL
	}

	path := strings.Trim(u.Path, "/")
	if u.Hostname() == "" || path == "" {
		return rawURL
	}

	return "git@" + u.Hostname() + ":" + path
}
