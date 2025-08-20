# 🚀 SFTP Web Client

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)]()

A modern, responsive web-based SFTP client built with Go. Manage your remote files through an intuitive web interface with support for file uploads, downloads, directory browsing, and bulk operations.

![SFTP Web Client Screenshot]([https://via.placeholder.com/800x400/2D3748/FFFFFF?text=SFTP+Web+Client](https://raw.githubusercontent.com/itsSwArchitect/sftp-server/refs/heads/main/sftp-server.png))

## ✨ Features

### 🎯 Core Functionality
- **Secure SFTP Connections** - Connect to any SFTP server with username/password authentication
- **Modern Web Interface** - Clean, responsive design with dark/light theme support
- **File Management** - Upload, download, delete files and directories
- **Directory Navigation** - Browse remote filesystem with breadcrumb navigation
- **Bulk Operations** - Select multiple files for download or deletion
- **File Preview** - Preview text files, code, and images directly in browser

### 🎨 User Interface
- **Grid & List Views** - Switch between grid and list layouts with persistent preferences
- **Responsive Design** - Works seamlessly on desktop, tablet, and mobile devices
- **Dark/Light Themes** - Toggle between themes with system preference detection
- **Drag & Drop Upload** - Simply drag files to upload them
- **Progress Indicators** - Real-time upload/download progress feedback

### 🔧 Advanced Features
- **Session Management** - Secure session handling with configurable timeouts
- **Login History** - Track recent connections for quick access
- **ZIP Downloads** - Download directories and multiple files as ZIP archives
- **File Filtering** - Filter files by type (images, documents, code, etc.)
- **Health Monitoring** - Built-in health check and monitoring endpoints

## 🚀 Quick Start

### Prerequisites
- Go 1.21 or higher
- Access to an SFTP server

### Installation

1. **Clone the repository:**
```bash
git clone https://github.com/yourusername/sftp-web-client.git
cd sftp-web-client
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Run the application:**
```bash
# Using go run (development)
go run ./cmd/sftpd -h localhost -p 8080

# Or build and run (production)
go build -o bin/sftpd ./cmd/sftpd
./bin/sftpd -h localhost -p 8080
```

4. **Open your browser:**
Navigate to `http://localhost:8080`

## 📖 Usage

### Basic Usage

1. **Start the server:**
```bash
go run ./cmd/sftpd -h localhost -p 8080
```

2. **Connect to SFTP server:**
   - Open `http://localhost:8080` in your browser
   - Enter your SFTP server details:
     - Host: Your SFTP server address
     - Port: SFTP port (usually 22)
     - Username: Your username
     - Password: Your password
   - Click "Connect"

3. **Manage files:**
   - Browse directories by clicking folder icons
   - Upload files by dragging and dropping or clicking the upload button
   - Download files by clicking the download icon
   - Switch between grid and list views using the toggle button
   - Use bulk operations by selecting multiple files

### Command Line Options

```bash
./sftpd [OPTIONS]

OPTIONS:
    -h <host>         Server host address (default: localhost)
    -p <port>         Server port (default: 8088)
    -config <path>    Path to configuration file
    -version          Show version information
    -help             Show help information
```

### Environment Variables

```bash
# Server Configuration
SFTP_HOST=localhost          # Server host
SFTP_PORT=8088              # Server port
SFTP_TLS_ENABLED=false      # Enable TLS
SFTP_CERT_FILE=cert.pem     # TLS certificate file
SFTP_KEY_FILE=key.pem       # TLS private key file

# Security
SFTP_SESSION_TIMEOUT=3600   # Session timeout in seconds
SFTP_MAX_UPLOAD_SIZE=32     # Max upload size in MB

# UI Configuration
SFTP_DEFAULT_VIEW=list      # Default view mode (list/grid)
SFTP_DEFAULT_THEME=light    # Default theme (light/dark)
SFTP_LOG_LEVEL=info         # Log level (debug, info, warn, error)
```

## 🏗️ Architecture

The application follows a clean, modular architecture:

```
sftp-web-client/
├── cmd/sftpd/              # Application entry point
│   └── main.go
├── internal/               # Private application code
│   ├── config/            # Configuration management
│   ├── handlers/          # HTTP request handlers
│   ├── middleware/        # HTTP middleware
│   ├── models/           # Data models
│   └── services/         # Business logic
├── pkg/                   # Public library code
│   └── utils/            # Utility functions
├── web/                   # Web assets
│   ├── static/           # Static files (CSS, JS, images)
│   └── templates/        # HTML templates
├── configs/              # Configuration files
└── go.mod               # Go module definition
```

### Key Components

- **Handlers**: HTTP request processing and routing
- **Services**: Business logic for SFTP operations, session management
- **Middleware**: Authentication, logging, security headers
- **Models**: Data structures for sessions, files, configuration
- **Templates**: HTML templates with Go templating

## 🔧 Configuration

### Configuration File

Create a `config.json` file:

```json
{
  "server": {
    "host": "localhost",
    "port": 8080,
    "tls_enabled": false,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "idle_timeout": "120s"
  },
  "security": {
    "session_timeout": "1h",
    "max_upload_size": "32MB",
    "session_cookie_name": "sftp_session",
    "session_cookie_secure": false
  },
  "ui": {
    "default_view": "list",
    "default_theme": "light",
    "max_preview_size": "1MB",
    "enable_batch_ops": true
  },
  "logging": {
    "level": "info",
    "format": "json"
  }
}
```

### TLS Configuration

To enable HTTPS:

1. Generate certificates:
```bash
# Self-signed certificate for development
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

2. Update configuration:
```json
{
  "server": {
    "tls_enabled": true,
    "cert_file": "cert.pem",
    "key_file": "key.pem"
  }
}
```

## 🔒 Security Features

- **Secure Sessions**: Session-based authentication with configurable timeouts
- **CSRF Protection**: Built-in CSRF protection for all forms
- **Secure Headers**: Security headers (HSTS, CSP, X-Frame-Options)
- **Input Validation**: Comprehensive input validation and sanitization
- **Path Traversal Protection**: Prevents directory traversal attacks
- **Rate Limiting**: Configurable rate limiting for API endpoints

## 🛠️ Development

### Prerequisites for Development
- Go 1.21+
- Git
- Make (optional)

### Setting up Development Environment

1. **Clone and setup:**
```bash
git clone https://github.com/yourusername/sftp-web-client.git
cd sftp-web-client
go mod download
```

2. **Run in development mode:**
```bash
go run ./cmd/sftpd -h localhost -p 8080
```

3. **Run tests:**
```bash
go test ./...
```

4. **Build for production:**
```bash
go build -ldflags "-X main.version=v1.0.0 -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o bin/sftpd ./cmd/sftpd
```

### Project Structure Details

- **`cmd/sftpd/`**: Main application entry point with server setup
- **`internal/handlers/`**: HTTP handlers for different endpoints
- **`internal/services/`**: Business logic for SFTP, sessions, file operations
- **`internal/middleware/`**: HTTP middleware for auth, logging, security
- **`internal/models/`**: Data models and structures
- **`internal/config/`**: Configuration management
- **`pkg/utils/`**: Reusable utility functions
- **`web/templates/`**: HTML templates
- **`web/static/`**: Static assets (CSS, JS, images)

## 📊 API Endpoints

### Public Endpoints
- `GET /` - Main application page (login/file browser)
- `POST /connect` - SFTP connection endpoint
- `GET /health` - Health check endpoint
- `GET /version` - Version information

### Protected Endpoints (require authentication)
- `GET /disconnect` - Logout endpoint
- `GET /download` - File/directory download
- `POST /download-multiple` - Bulk download as ZIP
- `POST /upload` - File upload
- `GET /preview` - File preview
- `POST /delete` - File/directory deletion

## 🚢 Deployment

### Docker Deployment

1. **Create Dockerfile:**
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o bin/sftpd ./cmd/sftpd

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/bin/sftpd .
COPY --from=builder /app/web ./web
COPY --from=builder /app/configs ./configs

EXPOSE 8080
CMD ["./sftpd", "-h", "0.0.0.0", "-p", "8080"]
```

2. **Build and run:**
```bash
docker build -t sftp-web-client .
docker run -p 8080:8080 sftp-web-client
```

### Systemd Service

Create `/etc/systemd/system/sftp-web-client.service`:

```ini
[Unit]
Description=SFTP Web Client
After=network.target

[Service]
Type=simple
User=sftp-web
WorkingDirectory=/opt/sftp-web-client
ExecStart=/opt/sftp-web-client/sftpd -h 0.0.0.0 -p 8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable sftp-web-client
sudo systemctl start sftp-web-client
```

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guidelines](CONTRIBUTING.md) for details.

### How to Contribute

1. **Fork the repository**
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Make your changes**: Implement your feature or bug fix
4. **Add tests**: Ensure your changes are well tested
5. **Commit your changes**: `git commit -m 'Add some amazing feature'`
6. **Push to the branch**: `git push origin feature/amazing-feature`
7. **Open a Pull Request**

### Development Guidelines

- Follow Go best practices and conventions
- Write tests for new functionality
- Update documentation for user-facing changes
- Use meaningful commit messages
- Ensure code passes all linting checks

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Credits and Acknowledgments

### Core Dependencies
- **[github.com/pkg/sftp](https://github.com/pkg/sftp)** - Pure Go SFTP client and server library
- **[golang.org/x/crypto](https://golang.org/x/crypto)** - Go cryptography libraries
- **[Tailwind CSS](https://tailwindcss.com)** - Utility-first CSS framework

### Inspiration and References
- **SFTP Protocol**: [RFC 4251](https://tools.ietf.org/html/rfc4251) - SSH Protocol Architecture
- **Web Security**: [OWASP](https://owasp.org/) - Security best practices
- **Go Best Practices**: [Effective Go](https://golang.org/doc/effective_go.html)

### Special Thanks
- The Go community for excellent libraries and tools
- Contributors who help improve this project
- Users who provide feedback and bug reports

### Third-Party Assets
- Icons: Various emoji and Unicode symbols
- Default styling: Tailwind CSS components
- Font loading: System fonts with fallbacks

## 📈 Project Stats

- **Language**: Go 1.21+
- **Lines of Code**: ~2,500+
- **Files**: 15+ source files
- **Test Coverage**: 80%+ (goal)
- **Dependencies**: Minimal, security-focused

---

**Made with ❤️ by the Abid**

*Star ⭐ this repository if you find it helpful!*
