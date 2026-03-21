# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in CacheStorm, please report it responsibly.

**DO NOT** open a public GitHub issue for security vulnerabilities.

### How to Report

1. Email: Send details to the repository maintainers via GitHub private vulnerability reporting
2. GitHub: Use [Security Advisories](https://github.com/CacheStorm/CacheStorm/security/advisories/new) to report privately

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

### Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial Assessment**: Within 1 week
- **Fix Release**: Within 2 weeks for critical issues

## Security Features

CacheStorm provides the following security features:

### Authentication
- `requirepass` - Password-based authentication for RESP protocol
- HTTP API password authentication with constant-time comparison
- Session-based authentication with secure token generation
- ACL system with per-user command/key/channel permissions

### Encryption
- TLS 1.2+ support with configurable certificate/key
- Hardened cipher suites (AEAD-only: AES-GCM, ChaCha20-Poly1305)
- Secure curve preferences (X25519, P256)

### Access Control
- HTTP API command blacklist (SHUTDOWN, FLUSHALL, DEBUG, CONFIG blocked)
- Rate limiting on HTTP API (per-IP)
- Request body size limits (10MB)
- Max header size limits (1MB)

### Input Validation
- Key name validation (null bytes rejected, 64KB max)
- Value size limits (512MB max)
- Bitmap offset bounds (0 to 2^32-1)
- JSON path depth limits (128 levels)
- Integer overflow protection
- RESP protocol bounds (1M array elements, 512MB bulk strings)

### Resource Protection
- Lua script execution timeout (5 seconds)
- PubSub channel limits (10,000 per subscriber)
- Function registry limits (1,000 libraries)
- Event listener limits
- Memory pressure-based eviction

## Security Best Practices

When deploying CacheStorm in production:

1. **Always enable authentication** - Set `requirepass` and HTTP `password`
2. **Enable TLS** - Configure `tls_cert_file` and `tls_key_file`
3. **Bind to specific interfaces** - Avoid `0.0.0.0` in production
4. **Use network segmentation** - Run behind a firewall
5. **Run as non-root user** - Use the provided Docker image or systemd service
6. **Monitor access logs** - Watch for authentication failures
7. **Keep updated** - Apply security patches promptly
8. **Limit admin API exposure** - Bind port 8080 to localhost only
