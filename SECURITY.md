# Security Policy

## Supported Versions

Use this section to tell people about which versions of your project are currently being supported with security updates.

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security vulnerability, please report it responsibly.

### How to Report

**Please do NOT report security vulnerabilities through public GitHub issues.**

Instead, please send an email to security@yourdomain.com with:

- A description of the vulnerability
- Steps to reproduce or proof of concept
- Potential impact assessment
- Suggested fix (if you have one)

### What to Expect

- **Acknowledgment**: We'll acknowledge your email within 48 hours
- **Assessment**: We'll assess the vulnerability within 5 business days
- **Updates**: We'll keep you informed of our progress
- **Resolution**: We'll work to fix the issue as quickly as possible
- **Credit**: We'll credit you in our security advisory (unless you prefer to remain anonymous)

### Security Best Practices

When using SFTP Web Client:

- Use strong passwords for SFTP connections
- Enable HTTPS in production environments
- Keep the application updated
- Use proper firewall rules
- Monitor access logs regularly
- Don't expose the application directly to the internet without proper security measures

### Security Features

- Session-based authentication with configurable timeouts
- CSRF protection for all forms
- Security headers (HSTS, CSP, X-Frame-Options)
- Input validation and sanitization
- Path traversal protection
- Rate limiting capabilities

Thank you for helping keep SFTP Web Client secure!
