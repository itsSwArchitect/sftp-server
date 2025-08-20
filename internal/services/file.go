package services

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"sftp-gui/internal/models"
	"sftp-gui/pkg/utils"
)

// FileService handles file operations
type FileService struct {
	sessionService *SessionService
}

// NewFileService creates a new file service
func NewFileService(sessionService *SessionService) *FileService {
	return &FileService{
		sessionService: sessionService,
	}
}

// ListFiles lists files in a directory
func (f *FileService) ListFiles(sessionID, dirPath string, showHidden bool, filter string) ([]models.FileInfo, error) {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// Clean the path
	if dirPath == "" {
		dirPath = session.HomeDir
	}
	dirPath = path.Clean(dirPath)

	// List directory contents
	fileInfos, err := session.SFTPClient.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []models.FileInfo
	for _, info := range fileInfos {
		// Skip hidden files if not requested
		if !showHidden && strings.HasPrefix(info.Name(), ".") {
			continue
		}

		// Apply filter
		if filter != "" && !f.matchesFilter(info.Name(), filter) {
			continue
		}

		fileInfo := models.FileInfo{
			Name:    info.Name(),
			Size:    info.Size(),
			Mode:    info.Mode(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
			Path:    path.Join(dirPath, info.Name()),
		}

		files = append(files, fileInfo)
	}

	// Sort files: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return files, nil
}

// GetFile downloads a single file
func (f *FileService) GetFile(sessionID, filePath string) (io.ReadCloser, *models.FileInfo, error) {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return nil, nil, err
	}

	// Get file info
	stat, err := session.SFTPClient.Stat(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.IsDir() {
		return nil, nil, fmt.Errorf("path is a directory")
	}

	// Open file
	file, err := session.SFTPClient.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	fileInfo := &models.FileInfo{
		Name:    stat.Name(),
		Size:    stat.Size(),
		Mode:    stat.Mode(),
		ModTime: stat.ModTime(),
		IsDir:   false,
		Path:    filePath,
	}

	return file, fileInfo, nil
}

// GetMultipleFiles creates a ZIP archive of multiple files
func (f *FileService) GetMultipleFiles(sessionID string, filePaths []string) (io.Reader, error) {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	for _, filePath := range filePaths {
		// Get file info
		stat, err := session.SFTPClient.Stat(filePath)
		if err != nil {
			continue // Skip files that can't be accessed
		}

		if stat.IsDir() {
			continue // Skip directories for now
		}

		// Open source file
		srcFile, err := session.SFTPClient.Open(filePath)
		if err != nil {
			continue // Skip files that can't be opened
		}

		// Create zip file entry
		fileName := filepath.Base(filePath)
		zipFile, err := zipWriter.Create(fileName)
		if err != nil {
			srcFile.Close()
			continue
		}

		// Copy file content
		_, err = io.Copy(zipFile, srcFile)
		srcFile.Close()

		if err != nil {
			continue // Skip files with copy errors
		}
	}

	zipWriter.Close()
	return &buf, nil
}

// DeleteFile deletes a single file or directory
func (f *FileService) DeleteFile(sessionID, filePath string) error {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Check if it's a directory
	stat, err := session.SFTPClient.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.IsDir() {
		// Remove directory (must be empty)
		return session.SFTPClient.RemoveDirectory(filePath)
	}

	// Remove file
	return session.SFTPClient.Remove(filePath)
}

// DeleteMultipleFiles deletes multiple files
func (f *FileService) DeleteMultipleFiles(sessionID string, filePaths []string) ([]string, []string) {
	var deleted, failed []string

	for _, filePath := range filePaths {
		if err := f.DeleteFile(sessionID, filePath); err != nil {
			failed = append(failed, filePath)
		} else {
			deleted = append(deleted, filePath)
		}
	}

	return deleted, failed
}

// PreviewFile gets file content for preview
func (f *FileService) PreviewFile(sessionID, filePath string, maxSize int64) (string, string, error) {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return "", "", err
	}

	// Get file info
	stat, err := session.SFTPClient.Stat(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.IsDir() {
		return "", "", fmt.Errorf("cannot preview directory")
	}

	if stat.Size() > maxSize {
		return "", "", fmt.Errorf("file too large for preview")
	}

	// Open and read file
	file, err := session.SFTPClient.Open(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return "", "", fmt.Errorf("failed to read file: %w", err)
	}

	// Determine language for syntax highlighting
	language := utils.GetLanguageFromExtension(filepath.Ext(filePath))

	return string(content), language, nil
}

// UploadFile uploads a file to the server
func (f *FileService) UploadFile(sessionID, destPath string, src io.Reader, overwrite bool) error {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return err
	}

	// Check if file exists
	if !overwrite {
		if _, err := session.SFTPClient.Stat(destPath); err == nil {
			return fmt.Errorf("file already exists")
		}
	}

	// Create destination file
	dstFile, err := session.SFTPClient.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	_, err = io.Copy(dstFile, src)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// CreateDirectory creates a new directory
func (f *FileService) CreateDirectory(sessionID, dirPath string) error {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return err
	}

	return session.SFTPClient.Mkdir(dirPath)
}

// GetBreadcrumbs generates breadcrumb navigation
func (f *FileService) GetBreadcrumbs(currentPath string) []models.Breadcrumb {
	if currentPath == "" || currentPath == "/" {
		return []models.Breadcrumb{
			{Name: "Home", Path: "/"},
		}
	}

	parts := strings.Split(strings.Trim(currentPath, "/"), "/")
	breadcrumbs := []models.Breadcrumb{
		{Name: "Home", Path: "/"},
	}

	currentFullPath := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		currentFullPath = path.Join(currentFullPath, part)
		breadcrumbs = append(breadcrumbs, models.Breadcrumb{
			Name: part,
			Path: "/" + currentFullPath,
		})
	}

	return breadcrumbs
}

// matchesFilter checks if a filename matches the given filter
func (f *FileService) matchesFilter(filename, filter string) bool {
	filename = strings.ToLower(filename)
	filter = strings.ToLower(filter)

	// Simple filter types
	switch filter {
	case "images":
		ext := filepath.Ext(filename)
		return utils.IsImageFile(ext)
	case "documents":
		ext := filepath.Ext(filename)
		return utils.IsDocumentFile(ext)
	case "archives":
		ext := filepath.Ext(filename)
		return utils.IsArchiveFile(ext)
	case "code":
		ext := filepath.Ext(filename)
		return utils.IsCodeFile(ext)
	default:
		// String contains filter
		return strings.Contains(filename, filter)
	}
}

// DownloadMultiple creates a ZIP archive of multiple files and streams it to the response
func (f *FileService) DownloadMultiple(sessionID string, filePaths []string, w io.Writer) error {
	session, err := f.sessionService.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Create ZIP writer
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, filePath := range filePaths {
		// Clean the file path
		cleanPath := filepath.Clean(filePath)

		// Get file info
		fileInfo, err := session.SFTPClient.Stat(cleanPath)
		if err != nil {
			// Log error but continue with other files
			continue
		}

		if fileInfo.IsDir() {
			// For directories, recursively add all files
			err = f.addDirectoryToZip(session, zipWriter, cleanPath, filepath.Base(cleanPath))
			if err != nil {
				// Log error but continue
				continue
			}
		} else {
			// For files, add directly
			err = f.addFileToZip(session, zipWriter, cleanPath, filepath.Base(cleanPath))
			if err != nil {
				// Log error but continue
				continue
			}
		}
	}

	return nil
}

// addFileToZip adds a single file to the ZIP archive
func (f *FileService) addFileToZip(session *models.Session, zipWriter *zip.Writer, filePath, zipPath string) error {
	// Open the remote file
	file, err := session.SFTPClient.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	// Create entry in ZIP
	zipFile, err := zipWriter.Create(zipPath)
	if err != nil {
		return fmt.Errorf("failed to create zip entry for %s: %w", zipPath, err)
	}

	// Copy file content to ZIP
	_, err = io.Copy(zipFile, file)
	if err != nil {
		return fmt.Errorf("failed to copy file %s to zip: %w", filePath, err)
	}

	return nil
}

// addDirectoryToZip recursively adds a directory to the ZIP archive
func (f *FileService) addDirectoryToZip(session *models.Session, zipWriter *zip.Writer, dirPath, zipPath string) error {
	// List directory contents
	files, err := session.SFTPClient.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	// Create directory entry in ZIP
	if zipPath != "" {
		_, err = zipWriter.Create(zipPath + "/")
		if err != nil {
			return fmt.Errorf("failed to create zip directory entry for %s: %w", zipPath, err)
		}
	}

	// Process each file in the directory
	for _, file := range files {
		remotePath := path.Join(dirPath, file.Name())
		localZipPath := zipPath + "/" + file.Name()

		if file.IsDir() {
			// Recursively add subdirectory
			err = f.addDirectoryToZip(session, zipWriter, remotePath, localZipPath)
			if err != nil {
				// Log error but continue
				continue
			}
		} else {
			// Add file
			err = f.addFileToZip(session, zipWriter, remotePath, localZipPath)
			if err != nil {
				// Log error but continue
				continue
			}
		}
	}

	return nil
}
