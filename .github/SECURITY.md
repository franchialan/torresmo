# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Torresmo, please report it responsibly:

1. **Do NOT** open a public issue
2. Email: franchialan@users.noreply.github.com
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We will respond within 48 hours and work with you to address the issue.

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |
| < latest | ❌       |

## Security Best Practices

When using Torresmo:

- Never commit secrets or tokens to your repository
- Use `.secrets` files (added to `.gitignore`) for workflow tokens
- Keep Docker/Podman updated
- Use the latest version of Torresmo
