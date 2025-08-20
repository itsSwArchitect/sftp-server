package main

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var (
	sessions   = make(map[string]*Session)
	mutex      sync.RWMutex
	lastLogins []LoginHistory
	loginMutex sync.RWMutex
)

type LoginHistory struct {
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	Username string    `json:"username"`
	LastUsed time.Time `json:"last_used"`
}

type Session struct {
	SSHClient  *ssh.Client
	SFTPClient *sftp.Client
	CreatedAt  time.Time
	HomeDir    string // Store user's home directory
}

type PageData struct {
	Connected     bool
	Error         string
	Success       string
	Path          string
	Files         []os.FileInfo
	View          string // "list", "grid", "detailed"
	HomeDir       string
	ShowHidden    bool
	Filter        string
	LastLogins    []LoginHistory
	TotalFiles    int
	FilteredFiles int
}

const indexHTML = `<!DOCTYPE html>
<html lang="en" class="h-full">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>SFTP Web Client</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script>
        tailwind.config = {
            darkMode: 'class',
        }
    </script>
    <link rel="icon" href="data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 100 100'><text y='.9em' font-size='90'>📁</text></svg>">
    <style>
        .file-icon { font-size: 1.2em; }
        .upload-zone { border: 2px dashed #cbd5e0; transition: all 0.3s; }
        .upload-zone:hover { border-color: #4299e1; background-color: #ebf8ff; }
        .upload-zone.dragover { border-color: #3182ce; background-color: #bee3f8; }
        /* Theme transitions */
        * {
            transition: background-color 0.3s ease, color 0.3s ease, border-color 0.3s ease;
        }
        .dark .upload-zone { border-color: #4a5568; }
        .dark .upload-zone:hover { border-color: #63b3ed; background-color: #2d3748; }
        .dark .upload-zone.dragover { border-color: #4299e1; background-color: #1a202c; }
    </style>
</head>
<body class="bg-gray-50 dark:bg-gray-900 min-h-screen transition-colors duration-300">
    <div class="container mx-auto px-4 py-8 max-w-7xl">
        <!-- Header -->
        <header class="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-6 mb-8">
            <div class="flex items-center justify-between">
                <div class="flex items-center space-x-3">
                    <span class="text-3xl">📁</span>
                    <div>
                        <h1 class="text-2xl font-bold text-gray-800 dark:text-white">SFTP Web Client</h1>
                        <p class="text-gray-600 dark:text-gray-400 text-sm">Secure file transfer and management</p>
                    </div>
                </div>
                <div class="flex items-center space-x-4">
                    <!-- Theme Toggle -->
                    <button onclick="toggleTheme()" class="bg-gray-200 dark:bg-gray-600 text-gray-800 dark:text-gray-200 px-4 py-2 rounded-lg hover:bg-gray-300 dark:hover:bg-gray-500 transition-colors">
                        <span class="dark:hidden">🌙 Dark</span>
                        <span class="hidden dark:inline">☀️ Light</span>
                    </button>
                    {{if .Connected}}
                    <span class="text-sm text-green-600 dark:text-green-400 bg-green-100 dark:bg-green-900 px-3 py-1 rounded-full">● Connected</span>
                    <form method="POST" action="/disconnect" class="inline">
                        <button type="submit" class="bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded-lg transition duration-200">
                            Disconnect
                        </button>
                    </form>
                    {{end}}
                </div>
            </div>
        </header>

        <!-- Alerts -->
        {{if .Error}}
        <div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-lg mb-6" id="error-alert">
            <div class="flex justify-between items-center">
                <div class="flex items-center">
                    <span class="mr-2">⚠️</span>
                    <span>{{.Error}}</span>
                </div>
                <button onclick="document.getElementById('error-alert').style.display='none'" class="text-red-500 hover:text-red-700">×</button>
            </div>
        </div>
        {{end}}
        
        {{if .Success}}
        <div class="bg-green-50 border border-green-200 text-green-700 px-4 py-3 rounded-lg mb-6" id="success-alert">
            <div class="flex justify-between items-center">
                <div class="flex items-center">
                    <span class="mr-2">✅</span>
                    <span>{{.Success}}</span>
                </div>
                <button onclick="document.getElementById('success-alert').style.display='none'" class="text-green-500 hover:text-green-700">×</button>
            </div>
        </div>
        {{end}}

        {{if not .Connected}}
        <!-- Connection Form -->
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm p-8 max-w-2xl mx-auto">
            <h2 class="text-xl font-semibold text-gray-800 dark:text-white mb-6">Connect to SFTP Server</h2>
            
            <!-- Quick Login from History -->
            {{if .LastLogins}}
            <div class="mb-6 p-4 bg-gray-50 dark:bg-gray-700 rounded-lg">
                <h3 class="text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Quick Connect (Recent Connections)</h3>
                <div class="grid grid-cols-1 gap-2">
                    {{range .LastLogins}}
                    <div class="flex items-center justify-between p-3 bg-white dark:bg-gray-600 rounded border dark:border-gray-500 hover:shadow-sm transition duration-200">
                        <div class="flex-1">
                            <div class="font-medium text-gray-800 dark:text-white">{{.Username}}@{{.Host}}:{{.Port}}</div>
                            <div class="text-xs text-gray-500 dark:text-gray-400">Last used: {{.LastUsed.Format "Jan 02, 2006 15:04"}}</div>
                        </div>
                        <button onclick="quickConnect('{{.Host}}', '{{.Port}}', '{{.Username}}')" 
                                class="px-3 py-1 text-sm bg-blue-100 dark:bg-blue-900 hover:bg-blue-200 dark:hover:bg-blue-800 text-blue-700 dark:text-blue-200 rounded transition duration-200">
                            Use
                        </button>
                    </div>
                    {{end}}
                </div>
            </div>
            {{end}}
            
            <form method="POST" action="/connect" class="space-y-4" id="connect-form">
                <div class="grid grid-cols-2 gap-4">
                    <div class="col-span-2 sm:col-span-1">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Host / IP Address</label>
                        <input type="text" name="host" id="host-input" required placeholder="192.168.1.100 or example.com"
                               class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500">
                    </div>
                    <div class="col-span-2 sm:col-span-1">
                        <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Port</label>
                        <input type="number" name="port" id="port-input" value="22" 
                               class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500">
                    </div>
                </div>
                <div>
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Username</label>
                    <div class="flex space-x-2">
                        <input type="text" name="username" id="username-input" required placeholder="your-username"
                               class="flex-1 px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500">
                        <button type="button" onclick="setRootUser()" 
                                class="px-4 py-2 bg-red-600 hover:bg-red-700 text-white text-sm rounded-lg transition duration-200">
                            🔑 Root
                        </button>
                    </div>
                </div>
                <div>
                    <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Password</label>
                    <input type="password" name="password" required placeholder="your-password"
                           class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-white rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500">
                </div>
                <button type="submit" 
                        class="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-3 rounded-lg transition duration-200">
                    Connect to Server
                </button>
            </form>
            
            <div class="mt-6 p-4 bg-gray-50 rounded-lg">
                <p class="text-sm text-gray-600 font-medium mb-2">Security Note:</p>
                <ul class="text-xs text-gray-500 dark:text-gray-400 space-y-1">
                    <li>• For local/development use only</li>
                    <li>• Credentials are not stored</li>
                    <li>• Host key verification disabled</li>
                </ul>
            </div>
        </div>
        
        {{else}}
        <!-- File Browser -->
        <div class="bg-white dark:bg-gray-800 rounded-lg shadow-sm">
            <!-- Breadcrumb & Actions -->
            <div class="border-b border-gray-200 dark:border-gray-700 p-6">
                <!-- Breadcrumb -->
                <div class="flex items-center justify-between mb-4">
                    <nav class="flex items-center space-x-2 text-sm">
                        <a href="/?path=/" class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium">🏠 Root</a>
                        {{if ne .Path "/"}}
                            {{$parts := split .Path "/"}}
                            {{range $i, $part := $parts}}
                                {{if $part}}
                                    <span class="text-gray-400 dark:text-gray-500">/</span>
                                    <span class="text-gray-700 dark:text-gray-300">{{$part}}</span>
                                {{end}}
                            {{end}}
                        {{end}}
                    </nav>
                    
                    <div class="text-sm text-gray-500 dark:text-gray-400">
                        {{if .FilteredFiles}}{{.FilteredFiles}} of {{.TotalFiles}} items{{else}}{{len .Files}} items{{end}}
                    </div>
                </div>
                
                <!-- Filtering and Search Bar -->
                <div class="mb-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                    <div class="flex flex-wrap gap-4 items-center">
                        <!-- Search Filter -->
                        <div class="flex-1 min-w-64">
                            <input type="text" id="file-filter" placeholder="Filter files and folders..." 
                                   value="{{.Filter}}"
                                   class="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 placeholder-gray-500 dark:placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 dark:focus:ring-blue-400"
                                   onkeyup="filterFiles()" onchange="updateFilter()">
                        </div>
                        
                        <!-- Toggle Options -->
                        <div class="flex items-center gap-4">
                            <label class="flex items-center">
                                <input type="checkbox" id="show-hidden" {{if .ShowHidden}}checked{{end}} 
                                       onchange="toggleHidden()" class="mr-2 text-blue-600 dark:text-blue-400 bg-gray-100 dark:bg-gray-700 border-gray-300 dark:border-gray-600 rounded focus:ring-blue-500 dark:focus:ring-blue-400 focus:ring-2">
                                <span class="text-sm text-gray-700 dark:text-gray-300">Show hidden files</span>
                            </label>
                            
                            <button onclick="clearFilter()" class="px-3 py-1 text-sm bg-gray-200 dark:bg-gray-600 hover:bg-gray-300 dark:hover:bg-gray-500 text-gray-700 dark:text-gray-200 rounded transition duration-200">
                                Clear Filter
                            </button>
                        </div>
                    </div>
                    
                    <!-- Quick Filters -->
                    <div class="mt-3 flex flex-wrap gap-2">
                        <button onclick="setFilter('images')" class="px-2 py-1 text-xs bg-blue-100 dark:bg-blue-900 hover:bg-blue-200 dark:hover:bg-blue-800 text-blue-700 dark:text-blue-300 rounded">
                            🖼️ Images
                        </button>
                        <button onclick="setFilter('documents')" class="px-2 py-1 text-xs bg-green-100 dark:bg-green-900 hover:bg-green-200 dark:hover:bg-green-800 text-green-700 dark:text-green-300 rounded">
                            📄 Documents
                        </button>
                        <button onclick="setFilter('archives')" class="px-2 py-1 text-xs bg-yellow-100 dark:bg-yellow-900 hover:bg-yellow-200 dark:hover:bg-yellow-800 text-yellow-700 dark:text-yellow-300 rounded">
                            📦 Archives
                        </button>
                        <button onclick="setFilter('code')" class="px-2 py-1 text-xs bg-purple-100 dark:bg-purple-900 hover:bg-purple-200 dark:hover:bg-purple-800 text-purple-700 dark:text-purple-300 rounded">
                            💻 Code
                        </button>
                    </div>
                </div>
                
                <!-- Batch Operations Bar -->
                <div id="batch-operations" class="hidden mb-4 p-4 bg-yellow-50 dark:bg-yellow-900 border border-yellow-200 dark:border-yellow-700 rounded-lg">
                    <div class="flex items-center justify-between">
                        <div class="flex items-center gap-4">
                            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
                                Selected: <span id="selected-count">0</span> items
                            </span>
                            <button onclick="selectAll()" class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300">
                                Select All
                            </button>
                            <button onclick="clearSelection()" class="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-300">
                                Clear Selection
                            </button>
                        </div>
                        <div class="flex items-center gap-2">
                            <button onclick="downloadSelected()" 
                                    class="px-3 py-1 bg-green-600 dark:bg-green-700 hover:bg-green-700 dark:hover:bg-green-600 text-white text-sm rounded transition duration-200">
                                📥 Download Selected
                            </button>
                            <button onclick="deleteSelected()" 
                                    class="px-3 py-1 bg-red-600 dark:bg-red-700 hover:bg-red-700 dark:hover:bg-red-600 text-white text-sm rounded transition duration-200">
                                🗑️ Delete Selected
                            </button>
                        </div>
                    </div>
                </div>
                
                <!-- Action Bar -->
                <div class="flex flex-wrap gap-4 items-center justify-between mb-4">
                    <div class="flex flex-wrap gap-4">
                        <!-- Parent Directory -->
                        {{if ne .Path .HomeDir}}
                        <a href="/?path={{if eq .Path "/" }}{{.HomeDir}}{{else}}{{dir .Path}}{{end}}&view={{.View}}&show_hidden={{.ShowHidden}}&filter={{.Filter}}" 
                           class="inline-flex items-center px-3 py-2 bg-gray-100 hover:bg-gray-200 text-gray-700 rounded-lg transition duration-200">
                            <span class="mr-2">↑</span> {{if eq .Path "/"}}Home{{else}}Parent Directory{{end}}
                        </a>
                        {{end}}
                        
                        <!-- Home Directory -->
                        {{if ne .Path .HomeDir}}
                        <a href="/?path={{.HomeDir}}&view={{.View}}&show_hidden={{.ShowHidden}}&filter={{.Filter}}" 
                           class="inline-flex items-center px-3 py-2 bg-blue-100 hover:bg-blue-200 text-blue-700 rounded-lg transition duration-200">
                            <span class="mr-2">🏠</span> Home
                        </a>
                        {{end}}
                        
                        <!-- Upload -->
                        <div class="flex items-center">
                            <form method="POST" action="/upload" enctype="multipart/form-data" class="flex items-center gap-2">
                                <input type="hidden" name="path" value="{{.Path}}">
                                <input type="hidden" name="view" value="{{.View}}">
                                <input type="hidden" name="show_hidden" value="{{.ShowHidden}}">
                                <input type="hidden" name="filter" value="{{.Filter}}">
                                <input type="file" name="file" required class="text-sm" multiple>
                                <button type="submit" 
                                        class="px-3 py-2 bg-green-600 hover:bg-green-700 text-white rounded-lg transition duration-200">
                                    Upload
                                </button>
                            </form>
                        </div>
                        
                        <!-- Create Folder -->
                        <form method="POST" action="/mkdir" class="flex items-center gap-2">
                            <input type="hidden" name="current_path" value="{{.Path}}">
                            <input type="hidden" name="view" value="{{.View}}">
                            <input type="hidden" name="show_hidden" value="{{.ShowHidden}}">
                            <input type="hidden" name="filter" value="{{.Filter}}">
                            <input type="text" name="folder_name" placeholder="New folder name" required
                                   class="px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500">
                            <button type="submit" 
                                    class="px-3 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition duration-200">
                                Create Folder
                            </button>
                        </form>
                    </div>
                    
                    <!-- View Options -->
                    <div class="flex items-center gap-2">
                        <span class="text-sm text-gray-600 dark:text-gray-400 mr-2">View:</span>
                        <a href="/?path={{.Path}}&view=list&show_hidden={{.ShowHidden}}&filter={{.Filter}}" 
                           class="px-3 py-1 text-sm rounded {{if eq .View "list"}}bg-blue-600 text-white{{else}}bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-500{{end}} transition duration-200">
                            📋 List
                        </a>
                        <a href="/?path={{.Path}}&view=grid&show_hidden={{.ShowHidden}}&filter={{.Filter}}" 
                           class="px-3 py-1 text-sm rounded {{if eq .View "grid"}}bg-blue-600 text-white{{else}}bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-500{{end}} transition duration-200">
                            🔲 Grid
                        </a>
                        <a href="/?path={{.Path}}&view=detailed&show_hidden={{.ShowHidden}}&filter={{.Filter}}" 
                           class="px-3 py-1 text-sm rounded {{if eq .View "detailed"}}bg-blue-600 text-white{{else}}bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-300 hover:bg-gray-300 dark:hover:bg-gray-500{{end}} transition duration-200">
                            📊 Detailed
                        </a>
                    </div>
                </div>
            
            <!-- File Display based on view type -->
            {{if eq .View "grid"}}
            <!-- Grid View -->
            <div class="p-6">
                <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
                    {{range .Files}}
                    <div class="file-card bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600 p-4 hover:shadow-md transition duration-200 relative">
                        <div class="text-center">
                            <!-- Selection checkbox -->
                            <div class="absolute top-2 left-2">
                                <input type="checkbox" class="file-checkbox" 
                                       data-path="{{cleanPath $.Path .Name}}" 
                                       data-name="{{.Name}}" 
                                       data-isdir="{{.IsDir}}"
                                       onchange="updateSelection()" 
                                       class="rounded">
                            </div>
                            
                            <div class="text-4xl mb-2">
                                {{if .IsDir}}📁{{else}}{{fileIcon .Name}}{{end}}
                            </div>
                            <div class="text-sm">
                                {{if .IsDir}}
                                    <a href="/?path={{cleanPath $.Path .Name}}&view={{$.View}}&show_hidden={{$.ShowHidden}}&filter={{$.Filter}}" 
                                       class="file-name text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium block truncate">{{.Name}}</a>
                                {{else}}
                                    <span class="file-name text-gray-900 dark:text-gray-100 block truncate">{{.Name}}</span>
                                {{end}}
                            </div>
                            {{if not .IsDir}}
                            <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">{{formatSize .Size}}</div>
                            {{end}}
                            <div class="mt-2 flex justify-center gap-1">
                                {{if not .IsDir}}
                                <button onclick="previewFile('{{cleanPath $.Path .Name}}', '{{.Name}}')" 
                                        class="text-xs text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 preview-btn">👁️</button>
                                <a href="/download?path={{cleanPath $.Path .Name}}" 
                                   class="text-xs text-green-600 hover:text-green-800">⬇️</a>
                                {{end}}
                                <form method="POST" action="/delete" class="inline" 
                                      onsubmit="return confirm('Delete {{.Name}}?')">
                                    <input type="hidden" name="path" value="{{cleanPath $.Path .Name}}">
                                    <input type="hidden" name="current_path" value="{{$.Path}}">
                                    <input type="hidden" name="view" value="{{$.View}}">
                                    <input type="hidden" name="show_hidden" value="{{$.ShowHidden}}">
                                    <input type="hidden" name="filter" value="{{$.Filter}}">
                                    <button type="submit" class="text-xs text-red-600 hover:text-red-800">🗑️</button>
                                </form>
                            </div>
                        </div>
                    </div>
                    {{else}}
                    <div class="col-span-full text-center py-12 text-gray-500 dark:text-gray-400">
                        <div class="flex flex-col items-center">
                            <span class="text-4xl mb-4">📂</span>
                            <p class="text-lg">This directory is empty</p>
                        </div>
                    </div>
                    {{end}}
                </div>
            </div>
            
            {{else if eq .View "detailed"}}
            <!-- Detailed Table View -->
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead class="bg-gray-50">
                        <tr>
                            <th class="px-3 py-3 text-left">
                                <input type="checkbox" id="select-all-detailed" onchange="toggleSelectAll()" class="rounded">
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Type</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Size</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Modified</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Permissions</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                        {{range .Files}}
                        <tr class="file-row hover:bg-gray-50 dark:hover:bg-gray-700">
                            <td class="px-3 py-4">
                                <input type="checkbox" class="file-checkbox rounded text-blue-600 dark:text-blue-400 bg-gray-100 dark:bg-gray-700 border-gray-300 dark:border-gray-600 focus:ring-blue-500 dark:focus:ring-blue-400 focus:ring-2" 
                                       data-path="{{cleanPath $.Path .Name}}" 
                                       data-name="{{.Name}}" 
                                       data-isdir="{{.IsDir}}"
                                       onchange="updateSelection()">
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="flex items-center">
                                    {{if .IsDir}}
                                        <span class="file-icon mr-3">📁</span>
                                        <a href="/?path={{cleanPath $.Path .Name}}&view={{$.View}}&show_hidden={{$.ShowHidden}}&filter={{$.Filter}}" 
                                           class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium">{{.Name}}</a>
                                    {{else}}
                                        <span class="file-icon mr-3">{{fileIcon .Name}}</span>
                                        <span class="text-gray-900 dark:text-gray-100">{{.Name}}</span>
                                    {{end}}
                                </div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                {{if .IsDir}}Directory{{else}}{{fileType .Name}}{{end}}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                {{if not .IsDir}}{{formatSize .Size}}{{else}}-{{end}}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                {{.ModTime.Format "Jan 02, 2006 15:04"}}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                {{printf "%s" .Mode}}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm space-x-2">
                                {{if not .IsDir}}
                                <button onclick="previewFile('{{cleanPath $.Path .Name}}', '{{.Name}}')" 
                                        class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium preview-btn">👁️ Preview</button>
                                <a href="/download?path={{cleanPath $.Path .Name}}" 
                                   class="text-green-600 dark:text-green-400 hover:text-green-800 dark:hover:text-green-300 font-medium">📥 Download</a>
                                {{end}}
                                <form method="POST" action="/delete" class="inline" 
                                      onsubmit="return confirm('Are you sure you want to delete {{.Name}}?')">
                                    <input type="hidden" name="path" value="{{cleanPath $.Path .Name}}">
                                    <input type="hidden" name="current_path" value="{{$.Path}}">
                                    <input type="hidden" name="view" value="{{$.View}}">
                                    <button type="submit" class="text-red-600 dark:text-red-400 hover:text-red-800 dark:hover:text-red-300 font-medium">🗑️ Delete</button>
                                </form>
                            </td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="6" class="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                                <div class="flex flex-col items-center">
                                    <span class="text-4xl mb-4">📂</span>
                                    <p class="text-lg">This directory is empty</p>
                                </div>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            
            {{else}}
            <!-- Default List View -->
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead class="bg-gray-50 dark:bg-gray-900">
                        <tr>
                            <th class="px-3 py-3 text-left">
                                <input type="checkbox" id="select-all-list" onchange="toggleSelectAll()" class="rounded text-blue-600 dark:text-blue-400 bg-gray-100 dark:bg-gray-700 border-gray-300 dark:border-gray-600 focus:ring-blue-500 dark:focus:ring-blue-400 focus:ring-2">
                            </th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Name</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Size</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Modified</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wider">Actions</th>
                        </tr>
                    </thead>
                    <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                        {{range .Files}}
                        <tr class="file-row hover:bg-gray-50 dark:hover:bg-gray-700">
                            <td class="px-3 py-4">
                                <input type="checkbox" class="file-checkbox rounded text-blue-600 dark:text-blue-400 bg-gray-100 dark:bg-gray-700 border-gray-300 dark:border-gray-600 focus:ring-blue-500 dark:focus:ring-blue-400 focus:ring-2" 
                                       data-path="{{cleanPath $.Path .Name}}" 
                                       data-name="{{.Name}}" 
                                       data-isdir="{{.IsDir}}"
                                       onchange="updateSelection()">
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="flex items-center">
                                    {{if .IsDir}}
                                        <span class="file-icon mr-3">📁</span>
                                        <a href="/?path={{cleanPath $.Path .Name}}&view={{$.View}}&show_hidden={{$.ShowHidden}}&filter={{$.Filter}}" 
                                           class="file-name text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium">{{.Name}}</a>
                                    {{else}}
                                        <span class="file-icon mr-3">{{fileIcon .Name}}</span>
                                        <span class="file-name text-gray-900 dark:text-gray-100">{{.Name}}</span>
                                    {{end}}
                                </div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                {{if not .IsDir}}{{formatSize .Size}}{{else}}-{{end}}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-500 dark:text-gray-400">
                                {{.ModTime.Format "Jan 02, 2006 15:04"}}
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm space-x-2">
                                {{if not .IsDir}}
                                <button onclick="previewFile('{{cleanPath $.Path .Name}}', '{{.Name}}')" 
                                        class="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 font-medium preview-btn">👁️ Preview</button>
                                <a href="/download?path={{cleanPath $.Path .Name}}" 
                                   class="text-green-600 dark:text-green-400 hover:text-green-800 dark:hover:text-green-300 font-medium">📥 Download</a>
                                {{end}}
                                <form method="POST" action="/delete" class="inline" 
                                      onsubmit="return confirm('Are you sure you want to delete {{.Name}}?')">
                                    <input type="hidden" name="path" value="{{cleanPath $.Path .Name}}">
                                    <input type="hidden" name="current_path" value="{{$.Path}}">
                                    <input type="hidden" name="view" value="{{$.View}}">
                                    <button type="submit" class="text-red-600 hover:text-red-800 font-medium">🗑️ Delete</button>
                                </form>
                            </td>
                        </tr>
                        {{else}}
                        <tr>
                            <td colspan="4" class="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                                <div class="flex flex-col items-center">
                                    <span class="text-4xl mb-4">📂</span>
                                    <p class="text-lg">This directory is empty</p>
                                    <p class="text-sm">Upload files or create folders to get started</p>
                                </div>
                            </td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
            {{end}}
        </div>
        {{end}}
    </div>
    
    <!-- File Preview Modal -->
    <div id="preview-modal" class="fixed inset-0 bg-gray-600 dark:bg-gray-900 bg-opacity-50 dark:bg-opacity-50 hidden flex items-center justify-center p-4 z-50">
        <div class="bg-white dark:bg-gray-800 rounded-lg max-w-4xl w-full max-h-full overflow-hidden flex flex-col">
            <div class="flex justify-between items-center p-4 border-b border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900">
                <h3 id="preview-title" class="text-lg font-semibold text-gray-900 dark:text-white">File Preview</h3>
                <button onclick="closePreview()" class="text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300">
                    <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                    </svg>
                </button>
            </div>
            <div class="flex-1 overflow-hidden">
                <div id="preview-loading" class="flex items-center justify-center h-32 hidden">
                    <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 dark:border-blue-400"></div>
                    <span class="ml-2 text-gray-600 dark:text-gray-400">Loading preview...</span>
                </div>
                <div id="preview-error" class="p-4 text-red-600 dark:text-red-400 hidden"></div>
                <pre id="preview-content" class="p-4 overflow-auto h-96 bg-gray-50 dark:bg-gray-900 text-sm font-mono whitespace-pre-wrap border-0 text-gray-900 dark:text-gray-100"></pre>
            </div>
            <div class="p-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900 flex justify-between items-center">
                <span id="preview-info" class="text-sm text-gray-600 dark:text-gray-400"></span>
                <div class="space-x-2">
                    <button onclick="closePreview()" class="px-4 py-2 bg-gray-300 dark:bg-gray-600 text-gray-700 dark:text-gray-200 rounded hover:bg-gray-400 dark:hover:bg-gray-500">Close</button>
                    <a id="preview-download" href="#" class="px-4 py-2 bg-blue-600 dark:bg-blue-500 text-white rounded hover:bg-blue-700 dark:hover:bg-blue-600">📥 Download</a>
                </div>
            </div>
        </div>
    </div>
    
    <script>
        // Auto-hide alerts after 8 seconds
        setTimeout(() => {
            const alerts = document.querySelectorAll('[id$="-alert"]');
            alerts.forEach(alert => {
                if (alert) alert.style.display = 'none';
            });
        }, 8000);
        
        // File upload drag & drop
        const uploadZone = document.querySelector('.upload-zone');
        if (uploadZone) {
            ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
                uploadZone.addEventListener(eventName, preventDefaults, false);
            });
            
            function preventDefaults(e) {
                e.preventDefault();
                e.stopPropagation();
            }
            
            ['dragenter', 'dragover'].forEach(eventName => {
                uploadZone.addEventListener(eventName, highlight, false);
            });
            
            ['dragleave', 'drop'].forEach(eventName => {
                uploadZone.addEventListener(eventName, unhighlight, false);
            });
            
            function highlight() {
                uploadZone.classList.add('dragover');
            }
            
            function unhighlight() {
                uploadZone.classList.remove('dragover');
            }
        }
        
        // Quick connect function
        function quickConnect(host, port, username) {
            document.getElementById('host-input').value = host;
            document.getElementById('port-input').value = port;
            document.getElementById('username-input').value = username;
            document.getElementById('username-input').focus();
        }
        
        function setRootUser() {
            document.getElementById('username-input').value = 'root';
            document.getElementById('username-input').focus();
        }
        
        // File filtering functions
        function filterFiles() {
            const filter = document.getElementById('file-filter').value.toLowerCase();
            const rows = document.querySelectorAll('.file-row');
            const cards = document.querySelectorAll('.file-card');
            
            // Filter table rows
            rows.forEach(row => {
                const fileName = row.querySelector('.file-name').textContent.toLowerCase();
                const shouldShow = fileName.includes(filter);
                row.style.display = shouldShow ? '' : 'none';
            });
            
            // Filter grid cards
            cards.forEach(card => {
                const fileName = card.querySelector('.file-name').textContent.toLowerCase();
                const shouldShow = fileName.includes(filter);
                card.style.display = shouldShow ? '' : 'none';
            });
        }
        
        function setFilter(type) {
            const filterInput = document.getElementById('file-filter');
            const filters = {
                'images': '\\.(jpg|jpeg|png|gif|bmp|webp|svg|ico)$',
                'documents': '\\.(pdf|doc|docx|txt|md|rtf|odt)$',
                'archives': '\\.(zip|rar|7z|tar|gz|bz2)$',
                'code': '\\.(js|html|css|py|go|java|cpp|c|php|rb)$'
            };
            
            if (filters[type]) {
                applyRegexFilter(filters[type]);
                filterInput.value = type + ' files';
            }
        }
        
        function applyRegexFilter(pattern) {
            const regex = new RegExp(pattern, 'i');
            const rows = document.querySelectorAll('.file-row');
            const cards = document.querySelectorAll('.file-card');
            
            rows.forEach(row => {
                const fileName = row.querySelector('.file-name').textContent;
                const isDir = row.querySelector('.file-name').href && row.querySelector('.file-name').href.includes('/?path=');
                const shouldShow = isDir || regex.test(fileName);
                row.style.display = shouldShow ? '' : 'none';
            });
            
            cards.forEach(card => {
                const fileName = card.querySelector('.file-name').textContent;
                const isDir = card.querySelector('.file-name').href && card.querySelector('.file-name').href.includes('/?path=');
                const shouldShow = isDir || regex.test(fileName);
                card.style.display = shouldShow ? '' : 'none';
            });
        }
        
        function clearFilter() {
            document.getElementById('file-filter').value = '';
            filterFiles();
        }
        
        function toggleHidden() {
            const showHidden = document.getElementById('show-hidden').checked;
            const url = new URL(window.location);
            url.searchParams.set('show_hidden', showHidden);
            window.location.href = url.toString();
        }
        
        function updateFilter() {
            const filter = document.getElementById('file-filter').value;
            const url = new URL(window.location);
            if (filter) {
                url.searchParams.set('filter', filter);
            } else {
                url.searchParams.delete('filter');
            }
            window.location.href = url.toString();
        }
        
        // Batch operations functions
        function updateSelection() {
            const checkboxes = document.querySelectorAll('.file-checkbox');
            const checkedBoxes = document.querySelectorAll('.file-checkbox:checked');
            const batchOps = document.getElementById('batch-operations');
            const selectedCount = document.getElementById('selected-count');
            
            if (checkedBoxes.length > 0) {
                batchOps.classList.remove('hidden');
                selectedCount.textContent = checkedBoxes.length;
            } else {
                batchOps.classList.add('hidden');
            }
            
            // Update select all checkbox states
            const selectAllList = document.getElementById('select-all-list');
            const selectAllDetailed = document.getElementById('select-all-detailed');
            const allChecked = checkboxes.length > 0 && checkedBoxes.length === checkboxes.length;
            
            if (selectAllList) selectAllList.checked = allChecked;
            if (selectAllDetailed) selectAllDetailed.checked = allChecked;
        }
        
        function toggleSelectAll() {
            const selectAll = event.target;
            const checkboxes = document.querySelectorAll('.file-checkbox');
            
            checkboxes.forEach(checkbox => {
                checkbox.checked = selectAll.checked;
            });
            
            updateSelection();
        }
        
        function selectAll() {
            const checkboxes = document.querySelectorAll('.file-checkbox');
            checkboxes.forEach(checkbox => {
                checkbox.checked = true;
            });
            updateSelection();
        }
        
        function clearSelection() {
            const checkboxes = document.querySelectorAll('.file-checkbox');
            checkboxes.forEach(checkbox => {
                checkbox.checked = false;
            });
            updateSelection();
        }
        
        function downloadSelected() {
            const checkedBoxes = document.querySelectorAll('.file-checkbox:checked');
            const filePaths = [];
            
            checkedBoxes.forEach(checkbox => {
                if (checkbox.dataset.isdir === 'false') { // Only download files, not directories
                    filePaths.push(checkbox.dataset.path);
                }
            });
            
            if (filePaths.length === 0) {
                alert('Please select at least one file to download (directories cannot be downloaded).');
                return;
            }
            
            if (filePaths.length === 1) {
                // Single file download
                window.location.href = '/download?path=' + encodeURIComponent(filePaths[0]);
            } else {
                // Multiple files download
                const form = document.createElement('form');
                form.method = 'POST';
                form.action = '/download-multiple';
                
                filePaths.forEach(path => {
                    const input = document.createElement('input');
                    input.type = 'hidden';
                    input.name = 'files[]';
                    input.value = path;
                    form.appendChild(input);
                });
                
                document.body.appendChild(form);
                form.submit();
                document.body.removeChild(form);
            }
        }
        
        function deleteSelected() {
            const checkedBoxes = document.querySelectorAll('.file-checkbox:checked');
            const filePaths = [];
            const fileNames = [];
            
            checkedBoxes.forEach(checkbox => {
                filePaths.push(checkbox.dataset.path);
                fileNames.push(checkbox.dataset.name);
            });
            
            if (filePaths.length === 0) {
                alert('Please select at least one item to delete.');
                return;
            }
            
            const fileList = fileNames.join(', ');
            if (!confirm('Are you sure you want to delete ' + filePaths.length + ' item(s)?\n\n' + fileList)) {
                return;
            }
            
            const form = document.createElement('form');
            form.method = 'POST';
            form.action = '/delete-multiple';
            
            filePaths.forEach(path => {
                const input = document.createElement('input');
                input.type = 'hidden';
                input.name = 'files[]';
                input.value = path;
                form.appendChild(input);
            });
            
            // Add current path for redirect
            const pathInput = document.createElement('input');
            pathInput.type = 'hidden';
            pathInput.name = 'current_path';
            pathInput.value = new URLSearchParams(window.location.search).get('path') || '/';
            form.appendChild(pathInput);
            
            // Add view parameters
            const viewInput = document.createElement('input');
            viewInput.type = 'hidden';
            viewInput.name = 'view';
            viewInput.value = new URLSearchParams(window.location.search).get('view') || 'list';
            form.appendChild(viewInput);
            
            const hiddenInput = document.createElement('input');
            hiddenInput.type = 'hidden';
            hiddenInput.name = 'show_hidden';
            hiddenInput.value = new URLSearchParams(window.location.search).get('show_hidden') || 'false';
            form.appendChild(hiddenInput);
            
            const filterInput = document.createElement('input');
            filterInput.type = 'hidden';
            filterInput.name = 'filter';
            filterInput.value = new URLSearchParams(window.location.search).get('filter') || '';
            form.appendChild(filterInput);
            
            document.body.appendChild(form);
            form.submit();
            document.body.removeChild(form);
        }

        // File preview functions
        function previewFile(filePath, fileName) {
            const modal = document.getElementById('preview-modal');
            const title = document.getElementById('preview-title');
            const loading = document.getElementById('preview-loading');
            const error = document.getElementById('preview-error');
            const content = document.getElementById('preview-content');
            const info = document.getElementById('preview-info');
            const downloadLink = document.getElementById('preview-download');
            
            // Reset modal state
            title.textContent = 'Preview: ' + fileName;
            loading.classList.remove('hidden');
            error.classList.add('hidden');
            content.classList.add('hidden');
            content.textContent = '';
            downloadLink.href = '/download?path=' + encodeURIComponent(filePath);
            
            // Show modal
            modal.classList.remove('hidden');
            
            // Fetch file content
            fetch('/preview?path=' + encodeURIComponent(filePath))
                .then(response => {
                    if (!response.ok) {
                        throw new Error('Failed to preview file: ' + response.statusText);
                    }
                    return response.text();
                })
                .then(text => {
                    loading.classList.add('hidden');
                    content.classList.remove('hidden');
                    content.textContent = text;
                    
                    // Update info
                    const lines = text.split('\n').length;
                    const size = new Blob([text]).size;
                    const sizeStr = size < 1024 ? size + ' B' : 
                                   size < 1024*1024 ? Math.round(size/1024) + ' KB' :
                                   Math.round(size/1024/1024) + ' MB';
                    info.textContent = lines + ' lines, ' + sizeStr;
                })
                .catch(err => {
                    loading.classList.add('hidden');
                    error.classList.remove('hidden');
                    error.textContent = err.message;
                });
        }
        
        function closePreview() {
            document.getElementById('preview-modal').classList.add('hidden');
        }
        
        // Close modal when clicking outside
        document.getElementById('preview-modal').addEventListener('click', function(e) {
            if (e.target === this) {
                closePreview();
            }
        });
        
        // Close modal with Escape key
        document.addEventListener('keydown', function(e) {
            if (e.key === 'Escape' && !document.getElementById('preview-modal').classList.contains('hidden')) {
                closePreview();
            }
        });

        // Theme management
        function initializeTheme() {
            const savedTheme = localStorage.getItem('theme');
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            
            if (savedTheme) {
                document.documentElement.classList.toggle('dark', savedTheme === 'dark');
            } else {
                document.documentElement.classList.toggle('dark', prefersDark);
            }
        }

        function toggleTheme() {
            const isDark = document.documentElement.classList.toggle('dark');
            localStorage.setItem('theme', isDark ? 'dark' : 'light');
        }

        // Initialize theme on page load
        initializeTheme();
    </script>
</body>
</html>`

// Template functions
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Login history management
func loadLoginHistory() {
	data, err := ioutil.ReadFile("login_history.json")
	if err != nil {
		return // File doesn't exist, start with empty history
	}

	loginMutex.Lock()
	defer loginMutex.Unlock()
	json.Unmarshal(data, &lastLogins)
}

func saveLoginHistory() {
	loginMutex.RLock()
	data, _ := json.MarshalIndent(lastLogins, "", "  ")
	loginMutex.RUnlock()

	ioutil.WriteFile("login_history.json", data, 0600)
}

func addLoginHistory(host string, port int, username string) {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	// Remove existing entry if present
	for i, login := range lastLogins {
		if login.Host == host && login.Port == port && login.Username == username {
			lastLogins = append(lastLogins[:i], lastLogins[i+1:]...)
			break
		}
	}

	// Add to beginning
	newLogin := LoginHistory{
		Host:     host,
		Port:     port,
		Username: username,
		LastUsed: time.Now(),
	}
	lastLogins = append([]LoginHistory{newLogin}, lastLogins...)

	// Keep only last 5 entries
	if len(lastLogins) > 5 {
		lastLogins = lastLogins[:5]
	}

	go saveLoginHistory()
}

// File filtering functions
func shouldShowFile(file os.FileInfo, showHidden bool, filter string) bool {
	name := file.Name()

	// Check hidden files
	if !showHidden && strings.HasPrefix(name, ".") {
		return false
	}

	// Apply filter
	if filter != "" {
		return strings.Contains(strings.ToLower(name), strings.ToLower(filter))
	}

	return true
}

func filterFiles(files []os.FileInfo, showHidden bool, filter string) ([]os.FileInfo, int, int) {
	totalFiles := len(files)
	var filtered []os.FileInfo

	for _, file := range files {
		if shouldShowFile(file, showHidden, filter) {
			filtered = append(filtered, file)
		}
	}

	return filtered, totalFiles, len(filtered)
}

func fileType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".md", ".log":
		return "Text File"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return "Image"
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv":
		return "Video"
	case ".mp3", ".wav", ".flac", ".aac":
		return "Audio"
	case ".pdf":
		return "PDF Document"
	case ".doc", ".docx":
		return "Word Document"
	case ".xls", ".xlsx":
		return "Excel Spreadsheet"
	case ".ppt", ".pptx":
		return "PowerPoint"
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return "Archive"
	case ".js", ".html", ".css", ".py", ".go", ".java", ".cpp", ".c":
		return "Source Code"
	default:
		return "File"
	}
}

func fileIcon(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".txt", ".md", ".log":
		return "📝"
	case ".pdf":
		return "📄"
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp":
		return "🖼️"
	case ".mp4", ".avi", ".mov", ".mkv":
		return "🎬"
	case ".mp3", ".wav", ".flac":
		return "🎵"
	case ".zip", ".tar", ".gz", ".rar":
		return "📦"
	case ".exe", ".bin":
		return "⚙️"
	case ".js", ".html", ".css", ".php", ".py", ".go", ".java":
		return "💻"
	default:
		return "📄"
	}
}

func cleanPath(currentPath, filename string) string {
	if currentPath == "/" {
		return "/" + filename
	}
	return currentPath + "/" + filename
}

func split(s, sep string) []string {
	return strings.Split(s, sep)
}

func dir(p string) string {
	return filepath.Dir(p)
}

var tmpl = template.Must(template.New("layout").Funcs(template.FuncMap{
	"formatSize": formatSize,
	"fileIcon":   fileIcon,
	"fileType":   fileType,
	"cleanPath":  cleanPath,
	"split":      split,
	"dir":        filepath.Dir,
}).Parse(indexHTML))

func generateSessionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func cleanupSessions() {
	ticker := time.NewTicker(30 * time.Minute)
	go func() {
		for range ticker.C {
			mutex.Lock()
			for id, session := range sessions {
				if time.Since(session.CreatedAt) > 2*time.Hour {
					session.SFTPClient.Close()
					session.SSHClient.Close()
					delete(sessions, id)
					log.Printf("Cleaned up expired session: %s", id)
				}
			}
			mutex.Unlock()
		}
	}()
}

func downloadMultipleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Parse form data first
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	sessionID := getSessionID(r)
	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	if session == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	filePaths := r.Form["files[]"]
	if len(filePaths) == 0 {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return
	}

	// Create a ZIP archive for multiple files
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="downloaded_files.zip"`)

	// Stream ZIP directly to response
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, filePath := range filePaths {
		file, err := session.SFTPClient.Open(filePath)
		if err != nil {
			log.Printf("Failed to open file %s: %v", filePath, err)
			continue
		}

		fileName := filepath.Base(filePath)
		zipFile, err := zipWriter.Create(fileName)
		if err != nil {
			file.Close()
			log.Printf("Failed to create zip entry for %s: %v", fileName, err)
			continue
		}

		_, err = io.Copy(zipFile, file)
		file.Close()

		if err != nil {
			log.Printf("Failed to copy file %s to zip: %v", fileName, err)
		}
	}
}

func deleteMultipleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Parse form data first
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	sessionID := getSessionID(r)
	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	if session == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	filePaths := r.Form["files[]"]
	currentPath := r.FormValue("current_path")
	view := r.FormValue("view")
	showHidden := r.FormValue("show_hidden")
	filter := r.FormValue("filter")

	if view == "" {
		view = "list"
	}

	var errors []string
	var deleted []string

	for _, filePath := range filePaths {
		// Try to remove as file first, then as directory
		err := session.SFTPClient.Remove(filePath)
		if err != nil {
			// If removing as file failed, try as directory
			err = session.SFTPClient.RemoveDirectory(filePath)
			if err != nil {
				errors = append(errors, filepath.Base(filePath))
				continue
			}
		}
		deleted = append(deleted, filepath.Base(filePath))
	}

	// Build redirect URL with parameters
	redirectURL := "/?path=" + currentPath + "&view=" + view + "&show_hidden=" + showHidden + "&filter=" + filter

	if len(errors) > 0 && len(deleted) > 0 {
		redirectURL += "&error=" + fmt.Sprintf("Deleted %d items, failed to delete: %s", len(deleted), strings.Join(errors, ", "))
	} else if len(errors) > 0 {
		redirectURL += "&error=" + fmt.Sprintf("Failed to delete: %s", strings.Join(errors, ", "))
	} else {
		redirectURL += "&success=" + fmt.Sprintf("Successfully deleted %d items", len(deleted))
	}

	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

func previewHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)
	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	if session == nil {
		http.Error(w, "No active session", http.StatusUnauthorized)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "No file path specified", http.StatusBadRequest)
		return
	}

	// Check if file is text-based
	if !isTextFile(filePath) {
		http.Error(w, "File is not a text file", http.StatusBadRequest)
		return
	}

	file, err := session.SFTPClient.Open(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Limit file size for preview (max 1MB)
	const maxPreviewSize = 1024 * 1024
	content := make([]byte, maxPreviewSize)
	n, err := file.Read(content)
	if err != nil && err != io.EOF {
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	content = content[:n]

	// Detect content type and language for syntax highlighting
	ext := strings.ToLower(filepath.Ext(filePath))
	language := getLanguageFromExtension(ext)

	// Return as JSON for AJAX
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"content":  string(content),
		"language": language,
		"filename": filepath.Base(filePath),
		"size":     n,
	}

	json.NewEncoder(w).Encode(response)
}

// Helper functions for file type detection
func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	textExtensions := map[string]bool{
		".txt": true, ".md": true, ".log": true, ".conf": true,
		".cfg": true, ".ini": true, ".yml": true, ".yaml": true,
		".json": true, ".xml": true, ".csv": true, ".sh": true,
		".bash": true, ".py": true, ".js": true, ".html": true,
		".htm": true, ".css": true, ".scss": true, ".sass": true,
		".go": true, ".java": true, ".cpp": true, ".c": true,
		".h": true, ".hpp": true, ".php": true, ".rb": true,
		".pl": true, ".sql": true, ".r": true, ".m": true,
		".swift": true, ".kt": true, ".rs": true, ".dart": true,
		".vue": true, ".jsx": true, ".tsx": true, ".ts": true,
		".dockerfile": true, ".gitignore": true, ".env": true,
	}

	// Check extension
	if textExtensions[ext] {
		return true
	}

	// Check for files without extension but common text file names
	basename := strings.ToLower(filepath.Base(filename))
	textBasenames := map[string]bool{
		"readme": true, "license": true, "changelog": true,
		"makefile": true, "dockerfile": true, "vagrantfile": true,
		"gemfile": true, "rakefile": true, ".gitignore": true,
		".htaccess": true, ".bashrc": true, ".zshrc": true,
		".vimrc": true, ".tmux.conf": true,
	}

	return textBasenames[basename]
}

func getLanguageFromExtension(ext string) string {
	languageMap := map[string]string{
		".js": "javascript", ".jsx": "javascript", ".ts": "typescript",
		".tsx": "typescript", ".py": "python", ".go": "go",
		".java": "java", ".cpp": "cpp", ".c": "c",
		".h": "c", ".hpp": "cpp", ".php": "php",
		".rb": "ruby", ".pl": "perl", ".sh": "bash",
		".bash": "bash", ".sql": "sql", ".html": "html",
		".htm": "html", ".css": "css", ".scss": "scss",
		".sass": "sass", ".json": "json", ".xml": "xml",
		".yml": "yaml", ".yaml": "yaml", ".md": "markdown",
		".swift": "swift", ".kt": "kotlin", ".rs": "rust",
		".dart": "dart", ".vue": "vue", ".r": "r",
		".m": "objective-c", ".dockerfile": "dockerfile",
	}

	if lang, exists := languageMap[ext]; exists {
		return lang
	}
	return "text"
}

func main() {
	// Load login history
	loadLoginHistory()

	// Start session cleanup routine
	cleanupSessions()

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/connect", connectHandler)
	http.HandleFunc("/disconnect", disconnectHandler)
	http.HandleFunc("/download", downloadHandler)
	http.HandleFunc("/download-multiple", downloadMultipleHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/mkdir", mkdirHandler)
	http.HandleFunc("/delete", deleteHandler)
	http.HandleFunc("/delete-multiple", deleteMultipleHandler)
	http.HandleFunc("/preview", previewHandler)

	fmt.Println("🚀 SFTP Web Client starting on http://localhost:8088")
	fmt.Println("📁 Open the URL in your browser to connect to your SFTP server")
	log.Fatal(http.ListenAndServe(":8088", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)

	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	// Get parameters
	view := r.URL.Query().Get("view")
	if view == "" {
		view = "list"
	}

	showHiddenStr := r.URL.Query().Get("show_hidden")
	showHidden := showHiddenStr == "true"

	filter := r.URL.Query().Get("filter")

	// Get login history for display
	loginMutex.RLock()
	loginHistory := make([]LoginHistory, len(lastLogins))
	copy(loginHistory, lastLogins)
	loginMutex.RUnlock()

	data := PageData{
		Connected:  session != nil,
		Path:       r.URL.Query().Get("path"),
		View:       view,
		ShowHidden: showHidden,
		Filter:     filter,
		LastLogins: loginHistory,
		Error:      r.URL.Query().Get("error"),
		Success:    r.URL.Query().Get("success"),
	}

	if data.Path == "" {
		if session != nil && session.HomeDir != "" {
			data.Path = session.HomeDir
		} else {
			data.Path = "/"
		}
	}

	if session != nil {
		data.HomeDir = session.HomeDir
		allFiles, err := session.SFTPClient.ReadDir(data.Path)
		if err != nil {
			data.Error = fmt.Sprintf("Failed to read directory: %v", err)
		} else {
			// Apply filtering
			filteredFiles, totalFiles, filteredCount := filterFiles(allFiles, showHidden, filter)
			data.Files = filteredFiles
			data.TotalFiles = totalFiles
			data.FilteredFiles = filteredCount
		}
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}

func connectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	host := r.FormValue("host")
	portStr := r.FormValue("port")
	username := r.FormValue("username")
	password := r.FormValue("password")

	port, _ := strconv.Atoi(portStr)
	if port == 0 {
		port = 22
	}

	// Connect to SSH
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), config)
	if err != nil {
		data := PageData{Error: fmt.Sprintf("SSH connection failed: %v", err)}
		w.Header().Set("Content-Type", "text/html")
		tmpl.Execute(w, data)
		return
	}

	// Open SFTP session
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		data := PageData{Error: fmt.Sprintf("SFTP session failed: %v", err)}
		w.Header().Set("Content-Type", "text/html")
		tmpl.Execute(w, data)
		return
	}

	// Store session
	sessionID := generateSessionID()

	// Add to login history
	addLoginHistory(host, port, username)

	// Try to detect user's home directory
	homeDir := "/"
	if wd, err := sftpClient.Getwd(); err == nil && wd != "" {
		homeDir = wd
	} else {
		// Try common home directory patterns
		if _, err := sftpClient.Stat("/home/" + username); err == nil {
			homeDir = "/home/" + username
		} else if _, err := sftpClient.Stat("/Users/" + username); err == nil {
			homeDir = "/Users/" + username
		}
	}

	mutex.Lock()
	sessions[sessionID] = &Session{
		SSHClient:  sshClient,
		SFTPClient: sftpClient,
		HomeDir:    homeDir,
		CreatedAt:  time.Now(),
	}
	mutex.Unlock()

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:  "session_id",
		Value: sessionID,
		Path:  "/",
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func disconnectHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)

	mutex.Lock()
	if session := sessions[sessionID]; session != nil {
		session.SFTPClient.Close()
		session.SSHClient.Close()
		delete(sessions, sessionID)
	}
	mutex.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := getSessionID(r)

	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	if session == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "No file path specified", http.StatusBadRequest)
		return
	}

	file, err := session.SFTPClient.Open(filePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open file: %v", err), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
	w.Header().Set("Content-Type", "application/octet-stream")

	io.Copy(w, file)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	sessionID := getSessionID(r)
	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	if session == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	currentPath := r.FormValue("path")
	view := r.FormValue("view")
	showHidden := r.FormValue("show_hidden")
	filter := r.FormValue("filter")

	if currentPath == "" {
		currentPath = "/"
	}
	if view == "" {
		view = "list"
	}

	redirectURL := fmt.Sprintf("/?path=%s&view=%s&show_hidden=%s&filter=%s", currentPath, view, showHidden, filter)

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, redirectURL+"&error="+fmt.Sprintf("Upload failed: %v", err), http.StatusSeeOther)
		return
	}
	defer file.Close()

	remotePath := path.Join(currentPath, header.Filename)
	if currentPath == "/" {
		remotePath = "/" + header.Filename
	}

	remoteFile, err := session.SFTPClient.Create(remotePath)
	if err != nil {
		http.Redirect(w, r, redirectURL+"&error="+fmt.Sprintf("Failed to create remote file: %v", err), http.StatusSeeOther)
		return
	}
	defer remoteFile.Close()

	_, err = io.Copy(remoteFile, file)
	if err != nil {
		http.Redirect(w, r, redirectURL+"&error="+fmt.Sprintf("Upload failed: %v", err), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, redirectURL+"&success=File uploaded successfully", http.StatusSeeOther)
}

func mkdirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	sessionID := getSessionID(r)
	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	if session == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	currentPath := r.FormValue("current_path")
	folderName := r.FormValue("folder_name")
	view := r.FormValue("view")
	showHidden := r.FormValue("show_hidden")
	filter := r.FormValue("filter")

	if view == "" {
		view = "list"
	}

	redirectURL := fmt.Sprintf("/?path=%s&view=%s&show_hidden=%s&filter=%s", currentPath, view, showHidden, filter)

	if folderName == "" {
		http.Redirect(w, r, redirectURL+"&error=Folder name cannot be empty", http.StatusSeeOther)
		return
	}

	newFolderPath := path.Join(currentPath, folderName)
	if currentPath == "/" {
		newFolderPath = "/" + folderName
	}

	err := session.SFTPClient.Mkdir(newFolderPath)
	if err != nil {
		http.Redirect(w, r, redirectURL+"&error="+fmt.Sprintf("Failed to create folder: %v", err), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, redirectURL+"&success=Folder created successfully", http.StatusSeeOther)
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	sessionID := getSessionID(r)
	mutex.RLock()
	session := sessions[sessionID]
	mutex.RUnlock()

	if session == nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	filePath := r.FormValue("path")
	currentPath := r.FormValue("current_path")
	view := r.FormValue("view")
	showHidden := r.FormValue("show_hidden")
	filter := r.FormValue("filter")

	if view == "" {
		view = "list"
	}

	redirectURL := fmt.Sprintf("/?path=%s&view=%s&show_hidden=%s&filter=%s", currentPath, view, showHidden, filter)

	if filePath == "" {
		http.Redirect(w, r, redirectURL+"&error=No file path specified", http.StatusSeeOther)
		return
	}

	// Try to remove as file first, then as directory
	err := session.SFTPClient.Remove(filePath)
	if err != nil {
		// If removing as file failed, try as directory
		err = session.SFTPClient.RemoveDirectory(filePath)
		if err != nil {
			http.Redirect(w, r, redirectURL+"&error="+fmt.Sprintf("Failed to delete: %v", err), http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, redirectURL+"&success=Item deleted successfully", http.StatusSeeOther)
}

func getSessionID(r *http.Request) string {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}
