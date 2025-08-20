# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2025-08-20

### Added
- Complete modular architecture with clean separation of concerns
- Modern responsive web interface with Tailwind CSS
- Grid and list view modes with persistent preferences
- Dark/light theme toggle with system preference detection
- Drag and drop file upload functionality
- Bulk file operations (select multiple files for download/delete)
- ZIP download for directories and multiple files
- File preview for text files, code, and images
- Real-time upload progress indicators
- Session management with configurable timeouts
- Login history tracking for quick reconnection
- Breadcrumb navigation for directory browsing
- File filtering by type (images, documents, code, etc.)
- Comprehensive error handling and user feedback
- Health check and monitoring endpoints
- Graceful server shutdown
- Security headers and CSRF protection
- Configuration management with environment variables
- Comprehensive logging and middleware stack

### Security
- Session-based authentication
- Input validation and sanitization
- Path traversal protection
- Secure cookie handling
- Rate limiting capabilities

### Technical
- Clean modular architecture following Go best practices
- Separation of handlers, services, middleware, and models
- Template-based HTML rendering
- SFTP client with proper connection management
- RESTful API design
- Comprehensive error handling
- Production-ready configuration management

### Documentation
- Complete README with usage examples
- API documentation
- Deployment guides (Docker, systemd)
- Contributing guidelines
- Security policy
- MIT License

## [0.1.0] - 2025-08-19

### Added
- Initial monolithic prototype (`simple-sftp.go`)
- Basic SFTP connection and file browsing
- Simple file upload/download functionality
- Basic HTML interface

### Note
This version was a single-file prototype that was completely refactored into the modular v1.0.0 architecture.
