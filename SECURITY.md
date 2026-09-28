# Security policy

## Reporting a vulnerability

Please do **not** open a public issue for security vulnerabilities.

Report privately through GitHub's private vulnerability reporting:
<https://github.com/juan7732/ergo/security/advisories/new>. Include the ergo
version (`ergo --version`), your platform, a description of the impact, and a
reproducer if you have one.

You will get an acknowledgement within a reasonable window, and disclosure will
be coordinated with you before any advisory is published.

## Supported versions

Only the latest tagged release receives security fixes. Fixes ship as a new
release; there are no backports.

## Scope

ergo is a local command-line tool with no service component. Things in scope:

- Unsafe handling of workspace or global configuration that could lead to
  command execution or path traversal outside the workspace root.
- Weaknesses in `ergo update` (release download, checksum verification,
  binary replacement).
- Any behavior that reads, stores, or transmits credentials. ergo is not
  supposed to touch them at all; see [PRIVACY.md](PRIVACY.md).

Vulnerabilities in `git`, `gh`, `code`, or the repositories a user chooses to
clone are out of scope and should go to those projects.
