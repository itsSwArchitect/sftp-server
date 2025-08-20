package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"sftp-gui/internal/config"
	"sftp-gui/internal/middleware"
	"sftp-gui/internal/models"
	"sftp-gui/internal/services"
)

// Handler holds all handler dependencies
type Handler struct {
	sessionService      *services.SessionService
	fileService         *services.FileService
	loginHistoryService *services.LoginHistoryService
	config              *config.Config
	templates           *template.Template
}

// New creates a new handler instance
func New(
	sessionService *services.SessionService,
	fileService *services.FileService,
	loginHistoryService *services.LoginHistoryService,
	cfg *config.Config,
	templates *template.Template,
) *Handler {
	return &Handler{
		sessionService:      sessionService,
		fileService:         fileService,
		loginHistoryService: loginHistoryService,
		config:              cfg,
		templates:           templates,
	}
}

// Home renders the login page or file browser based on connection status
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	// Check if already logged in
	var session *models.Session
	if cookie, err := r.Cookie(h.config.Security.SessionCookieName); err == nil {
		if sess, err := h.sessionService.GetSession(cookie.Value); err == nil {
			session = sess
		}
	}

	// Parse query parameters
	path := r.URL.Query().Get("path")
	view := r.URL.Query().Get("view")
	showHidden := r.URL.Query().Get("show_hidden") == "true"
	filter := r.URL.Query().Get("filter")
	errorMsg := r.URL.Query().Get("error")
	successMsg := r.URL.Query().Get("success")

	if view == "" {
		view = h.config.UI.DefaultView
	}

	// Get login history
	loginHistory := h.loginHistoryService.GetHistory()

	data := &models.PageData{
		Connected:    session != nil,
		Path:         path,
		View:         view,
		ShowHidden:   showHidden,
		Filter:       filter,
		LoginHistory: loginHistory,
		Error:        errorMsg,
		Success:      successMsg,
		Theme:        h.config.UI.DefaultTheme,
	}

	if session != nil {
		// User is connected - show file browser
		if path == "" {
			path = session.HomeDir
			if path == "" {
				path = "/"
			}
		}
		data.Path = path
		data.SessionInfo = session

		// Get files
		files, err := h.fileService.ListFiles(session.ID, path, showHidden, filter)
		if err != nil {
			data.Error = err.Error()
		} else {
			data.Files = files
		}

		// Generate breadcrumbs
		data.Breadcrumbs = h.fileService.GetBreadcrumbs(path)

		// Use browser template
		h.templates.ExecuteTemplate(w, "browser.html", data)
	} else {
		// User not connected - show login form
		h.templates.ExecuteTemplate(w, "index.html", data)
	}
}

// Login handles SFTP login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/?error=Invalid form data", http.StatusFound)
		return
	}

	port, err := strconv.Atoi(r.FormValue("port"))
	if err != nil {
		port = 22 // Default SSH port
	}

	loginReq := &models.LoginRequest{
		Host:     r.FormValue("host"),
		Port:     port,
		Username: r.FormValue("username"),
		Password: r.FormValue("password"),
	}

	// Create session
	session, err := h.sessionService.CreateSession(loginReq)
	if err != nil {
		// Record failed login
		h.loginHistoryService.AddLogin(loginReq.Host, loginReq.Port, loginReq.Username, false)
		http.Redirect(w, r, fmt.Sprintf("/?error=%s", err.Error()), http.StatusFound)
		return
	}

	// Record successful login
	h.loginHistoryService.AddLogin(loginReq.Host, loginReq.Port, loginReq.Username, true)

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.config.Security.SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.config.Security.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// Logout handles user logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get session ID from cookie
	if cookie, err := r.Cookie(h.config.Security.SessionCookieName); err == nil {
		// Delete session
		h.sessionService.DeleteSession(cookie.Value)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.config.Security.SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.config.Security.SessionCookieSecure,
	})

	http.Redirect(w, r, "/?success=Logged out successfully", http.StatusFound)
}

// Files renders the file browser
func (h *Handler) Files(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Parse query parameters
	path := r.URL.Query().Get("path")
	if path == "" {
		path = session.HomeDir
	}

	view := r.URL.Query().Get("view")
	if view == "" {
		view = h.config.UI.DefaultView
	}

	showHidden := r.URL.Query().Get("show_hidden") == "true"
	filter := r.URL.Query().Get("filter")

	// Get files
	files, err := h.fileService.ListFiles(session.ID, path, showHidden, filter)
	if err != nil {
		data := &models.PageData{
			Connected:   true,
			Error:       err.Error(),
			Path:        path,
			View:        view,
			ShowHidden:  showHidden,
			Filter:      filter,
			SessionInfo: session,
			Theme:       h.config.UI.DefaultTheme,
		}
		h.templates.ExecuteTemplate(w, "browser.html", data)
		return
	}

	// Generate breadcrumbs
	breadcrumbs := h.fileService.GetBreadcrumbs(path)

	data := &models.PageData{
		Connected:       true,
		Path:            path,
		Files:           files,
		Breadcrumbs:     breadcrumbs,
		View:            view,
		ShowHidden:      showHidden,
		Filter:          filter,
		TotalFiles:      len(files),
		FilteredFiles:   len(files),
		ShowBulkActions: h.config.UI.EnableBatchOps,
		SessionInfo:     session,
		Theme:           h.config.UI.DefaultTheme,
	}

	if err := h.templates.ExecuteTemplate(w, "browser.html", data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// Download handles file downloads
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	filePath := r.URL.Query().Get("file")
	if filePath == "" {
		filePath = r.URL.Query().Get("path")
	}
	if filePath == "" {
		http.Error(w, "File path required", http.StatusBadRequest)
		return
	}

	downloadType := r.URL.Query().Get("type")

	// Check if it's a directory download or if the path is a directory
	session, err := h.sessionService.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusUnauthorized)
		return
	}

	// Check if path is a directory
	stat, err := session.SFTPClient.Stat(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to access path: %v", err), http.StatusInternalServerError)
		return
	}

	if stat.IsDir() || downloadType == "directory" {
		// Set headers for ZIP download
		dirName := filepath.Base(filePath)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, dirName))

		// Download directory as ZIP
		err := h.fileService.DownloadMultiple(sessionID, []string{filePath}, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// For regular files, use the existing file download logic
	file, fileInfo, err := h.fileService.GetFile(sessionID, filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Set headers for file download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileInfo.Name))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size))

	// Stream file content
	if _, err := io.Copy(w, file); err != nil {
		// Log error but don't send response as headers are already sent
		fmt.Printf("Error streaming file: %v\n", err)
	}
}

// Preview handles file preview
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	if !ok {
		h.writeJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		h.writeJSONError(w, "File path required", http.StatusBadRequest)
		return
	}

	content, language, err := h.fileService.PreviewFile(sessionID, filePath, h.config.UI.MaxPreviewSize)
	if err != nil {
		h.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"content":  content,
		"language": language,
		"size":     len(content),
	}

	h.writeJSON(w, response)
}

// Delete handles file deletion
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	filePath := r.FormValue("path")
	if filePath == "" {
		http.Error(w, "File path required", http.StatusBadRequest)
		return
	}

	if err := h.fileService.DeleteFile(sessionID, filePath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to the current directory
	currentPath := r.FormValue("current_path")
	view := r.FormValue("view")
	redirectURL := fmt.Sprintf("/files?path=%s&view=%s&success=File deleted successfully", currentPath, view)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// DownloadMultiple creates a ZIP archive of multiple files
func (h *Handler) DownloadMultiple(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	session, ok := middleware.GetSessionFromContext(r.Context())
	if !ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		h.writeJSONError(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	filePaths := r.Form["files"]
	if len(filePaths) == 0 {
		h.writeJSONError(w, "No files specified", http.StatusBadRequest)
		return
	}

	// Set headers for ZIP download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="downloaded_files.zip"`)

	// Use file service to create ZIP archive
	err = h.fileService.DownloadMultiple(session.ID, filePaths, w)
	if err != nil {
		h.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Upload handles file uploads
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.writeJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID, ok := middleware.GetSessionIDFromContext(r.Context())
	if !ok {
		h.writeJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse multipart form (32MB max)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		h.writeJSONError(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeJSONError(w, "Failed to get file from form", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Get upload path
	uploadPath := r.FormValue("path")
	if uploadPath == "" {
		uploadPath = "/"
	}

	// Create destination file path
	destPath := filepath.Join(uploadPath, header.Filename)

	// Check if file should be overwritten (default: false)
	overwrite := r.FormValue("overwrite") == "true"

	// Upload file
	err = h.fileService.UploadFile(sessionID, destPath, file, overwrite)
	if err != nil {
		h.writeJSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success response
	response := models.APIResponse{
		Success: true,
		Message: "File uploaded successfully",
		Data: map[string]interface{}{
			"filename": header.Filename,
			"size":     header.Size,
			"path":     destPath,
		},
	}

	h.writeJSON(w, response)
}

// writeJSON writes a JSON response
func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

// writeJSONError writes a JSON error response
func (h *Handler) writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := models.APIResponse{
		Success: false,
		Error:   message,
	}

	json.NewEncoder(w).Encode(response)
}
