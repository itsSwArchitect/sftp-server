package utils

import (
	"fmt"
	"mime"
	"path/filepath"
	"strings"
)

// GetContentType returns the MIME type for a file extension
func GetContentType(ext string) string {
	ext = strings.ToLower(ext)
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}

// GetLanguageFromExtension returns the programming language for syntax highlighting
func GetLanguageFromExtension(ext string) string {
	ext = strings.ToLower(ext)

	languageMap := map[string]string{
		".js":         "javascript",
		".jsx":        "jsx",
		".ts":         "typescript",
		".tsx":        "tsx",
		".py":         "python",
		".go":         "go",
		".java":       "java",
		".c":          "c",
		".cpp":        "cpp",
		".cc":         "cpp",
		".cxx":        "cpp",
		".h":          "c",
		".hpp":        "cpp",
		".cs":         "csharp",
		".php":        "php",
		".rb":         "ruby",
		".rs":         "rust",
		".swift":      "swift",
		".kt":         "kotlin",
		".scala":      "scala",
		".sh":         "bash",
		".bash":       "bash",
		".zsh":        "bash",
		".fish":       "bash",
		".ps1":        "powershell",
		".sql":        "sql",
		".html":       "html",
		".htm":        "html",
		".xml":        "xml",
		".css":        "css",
		".scss":       "scss",
		".sass":       "sass",
		".less":       "less",
		".json":       "json",
		".yaml":       "yaml",
		".yml":        "yaml",
		".toml":       "toml",
		".ini":        "ini",
		".conf":       "ini",
		".cfg":        "ini",
		".md":         "markdown",
		".markdown":   "markdown",
		".tex":        "latex",
		".r":          "r",
		".R":          "r",
		".m":          "matlab",
		".pl":         "perl",
		".lua":        "lua",
		".vim":        "vim",
		".dockerfile": "dockerfile",
		".docker":     "dockerfile",
		".makefile":   "makefile",
		".mk":         "makefile",
		".cmake":      "cmake",
		".gradle":     "gradle",
		".groovy":     "groovy",
		".clj":        "clojure",
		".elm":        "elm",
		".ex":         "elixir",
		".exs":        "elixir",
		".erl":        "erlang",
		".hrl":        "erlang",
		".fs":         "fsharp",
		".fsx":        "fsharp",
		".ml":         "ocaml",
		".mli":        "ocaml",
		".hs":         "haskell",
		".lhs":        "haskell",
		".dart":       "dart",
		".v":          "verilog",
		".sv":         "systemverilog",
		".vhd":        "vhdl",
		".vhdl":       "vhdl",
	}

	if lang, exists := languageMap[ext]; exists {
		return lang
	}

	return "text"
}

// IsImageFile checks if the file extension is an image
func IsImageFile(ext string) bool {
	ext = strings.ToLower(ext)
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff", ".tif"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

// IsDocumentFile checks if the file extension is a document
func IsDocumentFile(ext string) bool {
	ext = strings.ToLower(ext)
	docExts := []string{".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".txt", ".rtf", ".odt", ".ods", ".odp"}
	for _, docExt := range docExts {
		if ext == docExt {
			return true
		}
	}
	return false
}

// IsArchiveFile checks if the file extension is an archive
func IsArchiveFile(ext string) bool {
	ext = strings.ToLower(ext)
	archiveExts := []string{".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tar.gz", ".tar.bz2", ".tar.xz"}
	for _, archExt := range archiveExts {
		if ext == archExt {
			return true
		}
	}
	return false
}

// IsCodeFile checks if the file extension is a code file
func IsCodeFile(ext string) bool {
	ext = strings.ToLower(ext)
	codeExts := []string{
		".js", ".jsx", ".ts", ".tsx", ".py", ".go", ".java", ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp",
		".cs", ".php", ".rb", ".rs", ".swift", ".kt", ".scala", ".sh", ".bash", ".zsh", ".fish",
		".ps1", ".sql", ".html", ".htm", ".xml", ".css", ".scss", ".sass", ".less", ".json",
		".yaml", ".yml", ".toml", ".ini", ".conf", ".cfg", ".md", ".markdown", ".tex", ".r",
		".R", ".m", ".pl", ".lua", ".vim", ".dockerfile", ".docker", ".makefile", ".mk",
		".cmake", ".gradle", ".groovy", ".clj", ".elm", ".ex", ".exs", ".erl", ".hrl",
		".fs", ".fsx", ".ml", ".mli", ".hs", ".lhs", ".dart", ".v", ".sv", ".vhd", ".vhdl",
	}
	for _, codeExt := range codeExts {
		if ext == codeExt {
			return true
		}
	}
	return false
}

// IsTextFile checks if the file is likely to be text-based
func IsTextFile(ext string) bool {
	return IsCodeFile(ext) || IsDocumentFile(ext) || ext == ".txt" || ext == ".log"
}

// FormatFileSize formats file size in human-readable format
func FormatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}

	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	units := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	return fmt.Sprintf("%.1f %s", float64(size)/float64(div), units[exp])
}

// CleanPath cleans and validates a file path
func CleanPath(basePath, relativePath string) string {
	if relativePath == "" {
		return basePath
	}

	cleaned := filepath.Join(basePath, relativePath)
	cleaned = filepath.Clean(cleaned)

	// Ensure the path doesn't go above the base path (security)
	if !strings.HasPrefix(cleaned, filepath.Clean(basePath)) {
		return basePath
	}

	return cleaned
}

// SanitizeFilename removes or replaces invalid characters in filenames
func SanitizeFilename(filename string) string {
	// Replace invalid characters with underscores
	invalid := []string{"<", ">", ":", "\"", "|", "?", "*", "/", "\\"}
	for _, char := range invalid {
		filename = strings.ReplaceAll(filename, char, "_")
	}

	// Trim whitespace and dots
	filename = strings.Trim(filename, " .")

	// Ensure filename is not empty
	if filename == "" {
		filename = "unnamed_file"
	}

	return filename
}
