package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

type FileInfo struct {
	Name       string `json:"name"`
	Size       string `json:"size"`
	SizeBytes  int64  `json:"sizeBytes"`
	ModTime    string `json:"modTime"`
	ModTimeRaw int64  `json:"modTimeRaw"`
	IsDir      bool   `json:"isDir"`
	Path       string `json:"path"`
}

type StorageInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Used  uint64 `json:"used"`
}

const (
	port    = "8097"
	rootDir = "/Volumes"
)

func formatSize(size int64) string {
	if size == 0 {
		return "-"
	}
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

const htmlTemplate = `<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <title>MistVault Explorer</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root { 
            --bg: #f8f9fa; --surface: #ffffff; --primary: #1a73e8; --secondary: #5f6368; 
            --text: #202124; --text-dim: #5f6368; --border: #dadce0; --hover: #f1f3f4; --selected: #e8f0fe;
        }
        body.dark-theme {
            --bg: #202124; --surface: #2d2e30; --primary: #8ab4f8; --secondary: #9aa0a6; 
            --text: #e8eaed; --text-dim: #9aa0a6; --border: #5f6368; --hover: #3c4043; --selected: #3b4252;
        }
        
        body { font-family: "Segoe UI", Roboto, "Helvetica Neue", sans-serif; background: var(--bg); color: var(--text); padding: 0; margin: 0; overflow: hidden; transition: background 0.3s, color 0.3s; user-select: none; -webkit-user-select: none; }
        input, textarea { user-select: text !important; -webkit-user-select: text !important; }
        .app-container { display: flex; height: 100vh; flex-direction: column; }
        
        header { background: var(--surface); padding: 10px 24px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; height: 48px;}
        .logo { display: flex; align-items: center; font-size: 1.25rem; color: var(--text); font-weight: 500; }
        .logo span { margin-right: 12px; font-size: 1.5rem; }
        .hamburger { cursor: pointer; padding-right: 12px; font-size: 1.5rem; user-select: none; }

        .main-area { display: flex; flex-grow: 1; overflow: hidden; }
        
        .sidebar { width: 240px; flex-shrink: 0; background: var(--surface); border-right: 1px solid var(--border); padding: 16px 0; display: flex; flex-direction: column; overflow-y: auto;}
        .sidebar-section { margin-top: 16px; }
        .sidebar-title { padding: 0 24px; font-size: 0.75rem; font-weight: bold; color: var(--text-dim); text-transform: uppercase; margin-bottom: 8px; }
        .nav-item { padding: 10px 24px; display: flex; align-items: center; justify-content: space-between; color: var(--text); text-decoration: none; font-weight: 500; font-size: 0.875rem; cursor: pointer;}
        .nav-item .nav-content { display: flex; align-items: center; }
        .nav-item:hover { background: var(--hover); }
        .nav-item.active { background: var(--selected); color: var(--primary); border-radius: 0 24px 24px 0; margin-right: 16px; }
        .nav-item i { margin-right: 16px; font-size: 1.2rem; font-style: normal; }
        .remove-fav { opacity: 0; cursor: pointer; color: var(--text-dim); }
        .nav-item:hover .remove-fav { opacity: 1; }

        .content-area { flex-grow: 1; display: flex; flex-direction: column; background: var(--bg); overflow: hidden; position: relative; min-width: 0; }
        
        .toolbar { padding: 12px 24px; display: flex; align-items: center; border-bottom: 1px solid var(--border); background: var(--surface); gap: 16px; flex-wrap: wrap; }
        .breadcrumb { font-size: 1.1rem; color: var(--text); display: flex; align-items: center; white-space: nowrap; overflow-x: auto; scrollbar-width: none; flex-grow: 1;}
        .breadcrumb::-webkit-scrollbar { display: none; }
        .breadcrumb span { cursor: pointer; border-radius: 4px; padding: 4px 8px; transition: background 0.2s; }
        .breadcrumb span:hover { background: var(--hover); }
        .breadcrumb .separator { margin: 0 4px; color: var(--text-dim); cursor: default; }
        .breadcrumb .separator:hover { background: transparent; }

        .btn { padding: 6px 12px; background: var(--surface); border: 1px solid var(--border); color: var(--text); border-radius: 4px; cursor: pointer; font-size: 0.875rem; font-weight: 500; transition: background 0.2s; display: flex; align-items: center; gap: 8px;}
        .btn:hover { background: var(--hover); }
        .btn.primary { background: var(--primary); color: white; border-color: var(--primary); }
        .btn.primary:hover { filter: brightness(1.1); }
        .btn.jellyfin { background: #8e24aa; color: white; border-color: #8e24aa; }
        .btn.jellyfin:hover { filter: brightness(1.1); }
        
        .file-list-container { flex-grow: 1; overflow-y: auto; overflow-x: auto; padding: 0 24px; position: relative; }
        .drag-overlay { display: none; position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(26,115,232,0.1); border: 2px dashed var(--primary); z-index: 50; align-items: center; justify-content: center; font-size: 1.5rem; color: var(--primary); pointer-events: none; }
        
        table { width: 100%; min-width: 600px; border-collapse: collapse; margin-top: 10px; table-layout: fixed; }
        thead { position: sticky; top: 0; background: var(--bg); z-index: 10; box-shadow: 0 1px 0 var(--border); }
        th { text-align: left; padding: 12px 16px; color: var(--text-dim); font-size: 0.875rem; font-weight: 500; cursor: pointer; user-select: none; }
        td { padding: 12px 16px; border-bottom: 1px solid var(--border); font-size: 0.875rem; cursor: default; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        tr:hover td { background: var(--hover); }
        tr.selected td { background: var(--selected); }
        
        .name-cell { display: flex; align-items: center; overflow: hidden; }
        .name-text { max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; user-select: none; -webkit-user-select: none; }
        .icon { width: 24px; font-size: 1.2rem; display: inline-flex; align-items: center; justify-content: center; margin-right: 16px; flex-shrink: 0; }
        .folder-icon { color: var(--primary); }
        .file-icon { color: var(--secondary); }
        
        .col-name { width: 100%; }
        .col-size { width: 120px; text-align: right; }
        .col-time { width: 160px; text-align: right; }
        .col-actions { width: 80px; text-align: right; }
        
        .action-btn { background: transparent; border: none; color: var(--text-dim); cursor: pointer; padding: 4px; border-radius: 4px; font-size: 1rem;}
        .action-btn:hover { background: var(--hover); color: var(--text); }

        ::-webkit-scrollbar { width: 8px; height: 8px; }
        ::-webkit-scrollbar-track { background: transparent; }
        ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 4px; }
        ::-webkit-scrollbar-thumb:hover { background: var(--text-dim); }

        .loading-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.2); display: none; align-items: center; justify-content: center; z-index: 100; backdrop-filter: blur(2px); }
        .spinner { width: 40px; height: 40px; border: 4px solid var(--border); border-top-color: var(--primary); border-radius: 50%; animation: spin 1s linear infinite; }
        @keyframes spin { to { transform: rotate(360deg); } }
        
        .empty-state { padding: 40px; text-align: center; color: var(--text-dim); }

        /* Modal Styles */
        .modal { display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 200; align-items: center; justify-content: center; backdrop-filter: blur(2px); }
        .modal-content { background: var(--surface); padding: 24px; border-radius: 8px; width: 400px; max-width: 90%; box-shadow: 0 4px 12px rgba(0,0,0,0.15); border: 1px solid var(--border); }
        .modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
        .modal-header h2 { margin: 0; font-size: 1.25rem; word-break: break-all; color: var(--text); }
        .close-btn { background: transparent; border: none; font-size: 1.5rem; cursor: pointer; color: var(--text-dim); }
        .close-btn:hover { color: var(--text); }
        .modal-body .meta-row { display: flex; margin-bottom: 12px; font-size: 0.9rem; }
        .modal-body .meta-row span:first-child { width: 80px; color: var(--text-dim); font-weight: 500; }
        .modal-body .meta-row span:last-child { flex-grow: 1; word-break: break-all; color: var(--text); }
        .modal-footer { margin-top: 24px; display: flex; justify-content: flex-end; gap: 12px; }

        /* Context Menu Styles */
        .context-menu { display: none; position: fixed; background: var(--surface); border: 1px solid var(--border); box-shadow: 0 2px 10px rgba(0,0,0,0.1); border-radius: 4px; padding: 4px 0; z-index: 150; min-width: 150px; }
        .context-menu-item { padding: 8px 16px; font-size: 0.875rem; cursor: pointer; color: var(--text); }
        .context-menu-item:hover { background: var(--hover); }
        .context-menu-item.danger { color: #d93025; }

        /* Multi-selection styles */
        .select-checkbox { width: 18px; height: 18px; cursor: pointer; margin-right: 12px; flex-shrink: 0; }
        .selection-toolbar { display: none; position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%); background: var(--surface); border: 1px solid var(--border); border-radius: 40px; padding: 8px 24px; box-shadow: 0 4px 20px rgba(0,0,0,0.2); z-index: 100; align-items: center; gap: 16px; }
        .selection-count { font-weight: bold; border-right: 1px solid var(--border); padding-right: 16px; margin-right: 8px; }

        /* Upload Manager */
        .upload-manager { position: fixed; bottom: 24px; right: 24px; width: 350px; background: var(--surface); border: 1px solid var(--border); border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.15); z-index: 200; display: none; flex-direction: column; overflow: hidden; max-height: 400px; }
        .upload-header { padding: 12px 16px; background: var(--primary); color: white; display: flex; justify-content: space-between; align-items: center; cursor: pointer; }
        .upload-list { overflow-y: auto; flex-grow: 1; }
        .upload-item { padding: 12px 16px; border-bottom: 1px solid var(--border); }
        .upload-item-info { display: flex; justify-content: space-between; margin-bottom: 6px; font-size: 0.8rem; }
        .upload-item-name { flex-grow: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 500; }
        .upload-progress-container { height: 6px; background: var(--hover); border-radius: 3px; overflow: hidden; margin-bottom: 4px; position: relative; }
        .upload-progress-bar { height: 100%; background: var(--primary); width: 0%; transition: width 0.3s; }
        .upload-item-meta { display: flex; justify-content: space-between; font-size: 0.75rem; color: var(--text-dim); }
        .upload-cancel-btn { cursor: pointer; color: var(--text-dim); font-size: 1.1rem; }
        .upload-cancel-btn:hover { color: #d93025; }

        /* Storage Section */
        .storage-section { padding: 16px 24px; border-top: 1px solid var(--border); margin-top: auto; }
        .storage-item { margin-bottom: 12px; }
        .storage-info { display: flex; justify-content: space-between; font-size: 0.75rem; margin-bottom: 4px; color: var(--text-dim); }
        .storage-bar-container { height: 6px; background: var(--hover); border-radius: 3px; overflow: hidden; }
        .storage-bar { height: 100%; background: var(--primary); }
        .storage-bar.warning { background: #f4b400; }
        .storage-bar.danger { background: #d93025; }

        /* Breadcrumb Truncation */
        .breadcrumb { max-width: 100%; overflow: hidden; position: relative; display: flex; align-items: center; }
        .breadcrumb-scroll { display: flex; align-items: center; overflow: hidden; text-overflow: ellipsis; }
        .breadcrumb-scroll span { flex-shrink: 0; }
        .breadcrumb-scroll span.truncated { color: var(--text-dim); cursor: pointer; padding: 4px; }
        
        @media (max-width: 600px) {
            .sidebar { position: fixed; z-index: 1000; height: 100%; }
        }
    </style>
</head>
<body>
    <div class="app-container">
        <header>
            <div class="logo">
                <div class="hamburger" onclick="toggleSidebar()">☰</div>
                <span>☁️</span> MistVault Explorer
            </div>
            <div style="display:flex; gap: 12px; align-items: center;">
                <span id="ping-display" style="font-size: 0.8rem; color: var(--text-dim); margin-right: 12px; font-family: monospace;">Ping: -- ms</span>
                <button class="btn" id="theme-btn" onclick="toggleTheme()">🌙 Dark</button>
                <input type="file" id="upload-input" style="display:none" multiple onchange="handleUpload(event)">
                <button class="btn primary" onclick="document.getElementById('upload-input').click()"><i>⬆️</i> Upload</button>
            </div>
        </header>
        
        <div class="main-area">
            <div class="sidebar">
                <div class="nav-item active" onclick="loadPath('/', true)" id="nav-home">
                    <div class="nav-content"><i>🏠</i> Home</div>
                </div>
                <div class="sidebar-section">
                    <div class="sidebar-title" onclick="toggleFavorites()" style="cursor: pointer; display: flex; justify-content: space-between; align-items: center;">
                        <span>Favorites</span>
                        <span id="fav-toggle-icon">▼</span>
                    </div>
                    <div id="favorites-list"></div>
                </div>
                <div class="storage-section" id="storage-list">
                    <div class="sidebar-title">Storage</div>
                </div>
            </div>
            
            <div class="content-area">
                <div class="toolbar">
                    <div style="display: flex; flex-grow: 1; align-items: center; border: 1px solid var(--border); border-radius: 4px; padding: 4px 8px; background: var(--bg); overflow: hidden; min-width: 0;">
                        <div id="breadcrumb" class="breadcrumb" style="flex-grow: 1; min-width: 0;"></div>
                        <input type="text" id="path-input" style="display: none; width: 100%; background: transparent; border: none; color: var(--text); outline: none; font-size: 1rem;" onkeydown="handlePathInput(event)">
                        <button class="action-btn" id="edit-path-btn" onclick="editPath()" title="Edit Path" style="margin-left: 8px;">✏️</button>
                        <button class="action-btn" onclick="copyCurrentPath()" title="Copy Path" style="margin-left: 4px;">📋</button>
                    </div>
                    <button class="btn" onclick="createFolder()">📁 New Folder</button>
                    <button class="btn" id="fav-btn" onclick="toggleFavorite()" title="Add to Favorites">⭐</button>
                    <button class="btn" id="paste-btn" style="display: none;" onclick="handlePaste()">📋 Paste</button>
                </div>

                <div class="file-list-container" id="drop-zone">
                    <div class="drag-overlay" id="drag-overlay">Drop files to upload</div>
                    <table id="file-table">
                        <thead>
                            <tr>
                                <th style="width: 40px;"><input type="checkbox" id="select-all" onclick="toggleSelectAll()"></th>
                                <th class="col-name" id="th-name" onclick="sortFiles('name')">Name ↑</th>
                                <th class="col-size" id="th-size" onclick="sortFiles('size')">Size</th>
                                <th class="col-time" id="th-time" onclick="sortFiles('time')">Modified</th>
                                <th class="col-actions"></th>
                            </tr>
                        </thead>
                        <tbody id="file-list">
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    </div>

    <div id="selection-toolbar" class="selection-toolbar">
        <span class="selection-count" id="selection-count">0 files selected</span>
        <button class="btn" onclick="copySelected()">📋 Copy</button>
        <button class="btn" onclick="cutSelected()">✂️ Move</button>
        <button class="btn danger" onclick="deleteSelected()" style="background: #d93025; color: white;">🗑️ Delete</button>
        <button class="btn" onclick="clearSelection()" style="border: none;">✕</button>
    </div>

    <div id="upload-manager" class="upload-manager">
        <div class="upload-header" onclick="toggleUploadManager()">
            <span id="upload-summary">Uploads (0)</span>
            <span id="upload-toggle-icon">▼</span>
        </div>
        <div class="upload-list" id="upload-list"></div>
    </div>

    <div id="loading" class="loading-overlay">
        <div class="spinner"></div>
    </div>

    <!-- Metadata Modal -->
    <div id="file-modal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h2 id="modal-filename">File Name</h2>
                <button class="close-btn" onclick="closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div id="modal-preview" style="text-align: center; margin-bottom: 16px; max-height: 300px; overflow: hidden; display: flex; align-items: center; justify-content: center;"></div>
                <div class="meta-row"><span>Type:</span><span id="modal-type"></span></div>
                <div class="meta-row"><span>Size:</span><span id="modal-size"></span></div>
                <div class="meta-row"><span>Modified:</span><span id="modal-time"></span></div>
                <div class="meta-row"><span>Path:</span><span id="modal-path"></span></div>
            </div>
            <div class="modal-footer">
                <button class="btn" onclick="closeModal()">Close</button>
                <button class="btn primary" id="modal-save-btn" style="display:none;">Save</button>
                <button class="btn primary" id="modal-download-btn">Download</button>
            </div>
        </div>
    </div>

    <!-- Context Menu -->
    <div id="context-menu" class="context-menu">
        <div class="context-menu-item" id="ctx-download">Download</div>
        <div class="context-menu-item" id="ctx-rename">Rename</div>
        <div class="context-menu-item" id="ctx-copy">Copy</div>
        <div class="context-menu-item" id="ctx-cut">Move (Cut)</div>
        <div class="context-menu-item danger" id="ctx-delete">Delete</div>
    </div>

    <script>
        let currentPath = '/';
        let currentSort = { col: 'name', asc: true };
        let currentFilesData = [];
        let clipboard = null;
        let selectedFiles = new Set();
        let activeUploads = {};
        let favorites = [];
        let currentTheme = '';

        const fileList = document.getElementById('file-list');
        const breadcrumb = document.getElementById('breadcrumb');
        const loading = document.getElementById('loading');
        const favList = document.getElementById('favorites-list');
        const favBtn = document.getElementById('fav-btn');
        const pasteBtn = document.getElementById('paste-btn');
        const contextMenu = document.getElementById('context-menu');
        const dropZone = document.getElementById('drop-zone');
        const dragOverlay = document.getElementById('drag-overlay');
        const selectionToolbar = document.getElementById('selection-toolbar');
        const selectionCountText = document.getElementById('selection-count');
        const storageList = document.getElementById('storage-list');

        function toggleSidebar() {
            const sidebar = document.querySelector('.sidebar');
            sidebar.style.display = sidebar.style.display === 'none' ? 'flex' : 'none';
        }

        function sortFiles(col) {
            if (currentSort.col === col) {
                currentSort.asc = !currentSort.asc;
            } else {
                currentSort.col = col;
                currentSort.asc = true;
            }
            document.querySelectorAll('th').forEach(th => {
                if (th.id) th.textContent = th.textContent.replace(' ↑', '').replace(' ↓', '');
            });
            const th = document.getElementById('th-' + col);
            if (th) th.textContent += (currentSort.asc ? ' ↑' : ' ↓');
            renderFiles(currentFilesData);
        }

        async function loadSettings() {
            try {
                const res = await fetch('/api/settings');
                const data = await res.json();
                favorites = data.favorites || [];
                currentTheme = data.theme || '';
                
                if (currentTheme === 'dark' || (!currentTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
                    document.body.classList.add('dark-theme');
                    document.getElementById('theme-btn').textContent = '☀️ Light';
                } else {
                    document.body.classList.remove('dark-theme');
                    document.getElementById('theme-btn').textContent = '🌙 Dark';
                }
                
                renderFavorites();
                updateFavButton();
            } catch (e) {
                console.error("Failed to load settings", e);
            }
        }
        
        async function saveSettingsToServer() {
            try {
                await fetch('/api/settings', {
                    method: 'POST',
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ theme: currentTheme, favorites: favorites })
                });
            } catch (e) {
                console.error("Failed to save settings", e);
            }
        }

        function toggleTheme() {
            const isDark = document.body.classList.toggle('dark-theme');
            currentTheme = isDark ? 'dark' : 'light'; 
            saveSettingsToServer();
            document.getElementById('theme-btn').textContent = isDark ? '☀️ Light' : '🌙 Dark';
        }

        async function loadStorageInfo() {
            try {
                const res = await fetch('/api/storage');
                const data = await res.json();
                renderStorage(data);
            } catch (e) { console.error("Failed to load storage info", e); }
        }

        function renderStorage(data) {
            const title = storageList.querySelector('.sidebar-title');
            storageList.innerHTML = '';
            storageList.appendChild(title);
            data.forEach(item => {
                const usedPct = (item.used / item.total) * 100;
                const div = document.createElement('div');
                div.className = 'storage-item';
                div.innerHTML = '<div class="storage-info">' +
                        '<span>' + item.name + '</span>' +
                        '<span>' + formatBytes(item.used) + ' / ' + formatBytes(item.total) + '</span>' +
                    '</div>' +
                    '<div class="storage-bar-container">' +
                        '<div class="storage-bar ' + (usedPct > 90 ? 'danger' : usedPct > 75 ? 'warning' : '') + '" style="width: ' + usedPct + '%"></div>' +
                    '</div>';
                storageList.appendChild(div);
            });
        }

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
        }

        window.addEventListener('popstate', (event) => {
            const path = new URLSearchParams(window.location.search).get('path') || '/';
            loadPath(path, false);
        });

        function editPath() {
            const bc = document.getElementById('breadcrumb');
            const input = document.getElementById('path-input');
            const isEditing = input.style.display === 'block';
            
            if (isEditing) {
                cancelPathEdit();
            } else {
                bc.style.display = 'none';
                input.style.display = 'block';
                input.value = currentPath;
                input.focus();
            }
        }

        function handlePathInput(e) {
            if (e.key === 'Enter') {
                loadPath(document.getElementById('path-input').value, true);
                cancelPathEdit();
            } else if (e.key === 'Escape') {
                cancelPathEdit();
            }
        }

        function cancelPathEdit() {
            document.getElementById('breadcrumb').style.display = 'flex';
            document.getElementById('path-input').style.display = 'none';
        }

        document.addEventListener('click', (e) => {
            const input = document.getElementById('path-input');
            if (input && input.style.display === 'block' && e.target !== input && !e.target.closest('#breadcrumb') && !e.target.closest('.action-btn')) {
                cancelPathEdit();
            }
            contextMenu.style.display = 'none';
        });

        function copyCurrentPath() {
            navigator.clipboard.writeText(currentPath).then(() => {
                alert('경로가 복사되었습니다!');
            });
        }

        async function loadPath(path, push = true) {
            loading.style.display = 'flex';
            clearSelection();
            try {
                const response = await fetch('/api/list?path=' + encodeURIComponent(path));
                if (!response.ok) throw new Error('Network response was not ok');
                const data = await response.json();
                
                currentPath = data.currentPath || '/';
                renderBreadcrumb(currentPath);
                currentFilesData = data.files || []; 
                renderFiles(currentFilesData);
                updateFavButton();
                loadStorageInfo();
                
                if (push) {
                    const newUrl = currentPath === '/' ? window.location.pathname : window.location.pathname + '?path=' + encodeURIComponent(currentPath);
                    window.history.pushState({ path: currentPath }, '', newUrl);
                }
            } catch (err) {
                console.error('Failed to load path:', err);
                fileList.innerHTML = '<tr><td colspan="5" class="empty-state">경로를 로드할 수 없습니다.</td></tr>';
            } finally {
                loading.style.display = 'none';
            }
        }

        function renderBreadcrumb(path) {
            const parts = path.split('/').filter(p => p);
            breadcrumb.innerHTML = '';
            
            const bcScroll = document.createElement('div');
            bcScroll.className = 'breadcrumb-scroll';
            bcScroll.style.cssText = 'display: flex; align-items: center; overflow: hidden; white-space: nowrap; flex-grow: 1; mask-image: linear-gradient(to right, transparent, black 20px); -webkit-mask-image: linear-gradient(to right, transparent, black 20px);';
            breadcrumb.appendChild(bcScroll);

            const rootSpan = document.createElement('span');
            rootSpan.textContent = 'MistVault';
            rootSpan.onclick = (e) => { e.stopPropagation(); loadPath('/', true); };
            bcScroll.appendChild(rootSpan);

            let accumulatedPath = '';
            parts.forEach((part, index) => {
                accumulatedPath += '/' + part;
                const thisPath = accumulatedPath;
                const sepSpan = document.createElement('span');
                sepSpan.className = 'separator';
                sepSpan.textContent = '›';
                bcScroll.appendChild(sepSpan);
                const partSpan = document.createElement('span');
                partSpan.textContent = part;
                partSpan.onclick = (e) => { e.stopPropagation(); loadPath(thisPath, true); };
                bcScroll.appendChild(partSpan);
            });

            setTimeout(() => {
                bcScroll.scrollLeft = bcScroll.scrollWidth;
            }, 0);
        }

        function renderFiles(filesData) {
            let files = [...filesData];
            files.sort((a, b) => {
                if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
                let result = 0;
                if (currentSort.col === 'name') {
                    result = a.name.toLowerCase().localeCompare(b.name.toLowerCase());
                } else if (currentSort.col === 'size') {
                    result = a.sizeBytes - b.sizeBytes;
                } else if (currentSort.col === 'time') {
                    result = a.modTimeRaw - b.modTimeRaw;
                }
                return currentSort.asc ? result : -result;
            });

            fileList.innerHTML = '';
            document.getElementById('select-all').checked = false;

            if (currentPath !== '/' && currentPath !== '') {
                const parentPath = currentPath.substring(0, currentPath.lastIndexOf('/')) || '/';
                const tr = document.createElement('tr');
                tr.innerHTML = '<td></td><td colspan="4"><div class="name-cell"><span class="icon folder-icon">📁</span><span class="name-text">..</span></div></td>';
                tr.ondblclick = () => loadPath(parentPath, true);
                fileList.appendChild(tr);
            }

            if (files.length === 0) {
                const tr = document.createElement('tr');
                tr.innerHTML = '<td colspan="5" class="empty-state">이 폴더는 비어 있습니다.</td>';
                fileList.appendChild(tr);
                return;
            }

            files.forEach(file => {
                const tr = document.createElement('tr');
                tr.dataset.path = file.path;
                if (selectedFiles.has(file.path)) tr.classList.add('selected');

                const tdCheck = document.createElement('td');
                const cb = document.createElement('input');
                cb.type = 'checkbox';
                cb.className = 'select-checkbox';
                cb.checked = selectedFiles.has(file.path);
                cb.onclick = (e) => { e.stopPropagation(); toggleFileSelection(file.path, cb.checked); };
                tdCheck.appendChild(cb);

                const tdName = document.createElement('td');
                const nameDiv = document.createElement('div');
                nameDiv.className = 'name-cell';
                const clickableWrapper = document.createElement('div');
                clickableWrapper.style.display = 'flex';
                clickableWrapper.style.alignItems = 'center';
                clickableWrapper.style.maxWidth = '100%';
                clickableWrapper.style.width = 'fit-content';
                clickableWrapper.style.cursor = 'pointer';
                clickableWrapper.onclick = (e) => {
                    e.stopPropagation();
                    if (file.isDir) loadPath(file.path, true);
                    else showModal(file);
                };

                const iconSpan = document.createElement('span');
                iconSpan.className = 'icon ' + (file.isDir ? 'folder-icon' : 'file-icon');
                iconSpan.textContent = file.isDir ? '📁' : '📄';
                const nameSpan = document.createElement('span');
                nameSpan.className = 'name-text';
                nameSpan.textContent = file.name;
                
                clickableWrapper.appendChild(iconSpan);
                clickableWrapper.appendChild(nameSpan);
                nameDiv.appendChild(clickableWrapper);
                tdName.appendChild(nameDiv);
                
                const tdSize = document.createElement('td');
                tdSize.className = 'col-size';
                tdSize.textContent = file.size;
                
                const tdTime = document.createElement('td');
                tdTime.className = 'col-time';
                tdTime.textContent = file.modTime;

                const tdActions = document.createElement('td');
                tdActions.className = 'col-actions';
                const actionBtn = document.createElement('button');
                actionBtn.className = 'action-btn';
                actionBtn.innerHTML = '⋮';
                actionBtn.onclick = (e) => {
                    e.stopPropagation();
                    showContextMenu(e.clientX, e.clientY, file);
                };
                tdActions.appendChild(actionBtn);
                
                tr.appendChild(tdCheck);
                tr.appendChild(tdName);
                tr.appendChild(tdSize);
                tr.appendChild(tdTime);
                tr.appendChild(tdActions);
                
                tr.ondblclick = () => file.isDir ? loadPath(file.path, true) : showModal(file);
                tr.onclick = (e) => {
                    if (e.ctrlKey || e.metaKey) {
                        toggleFileSelection(file.path, !selectedFiles.has(file.path));
                    } else {
                        clearSelection();
                        toggleFileSelection(file.path, true);
                    }
                };
                tr.oncontextmenu = (e) => {
                    e.preventDefault();
                    if (!selectedFiles.has(file.path)) {
                        clearSelection();
                        toggleFileSelection(file.path, true);
                    }
                    showContextMenu(e.clientX, e.clientY, file);
                };

                fileList.appendChild(tr);
            });
        }

        function toggleFileSelection(path, selected) {
            if (selected) selectedFiles.add(path);
            else selectedFiles.delete(path);
            
            document.querySelectorAll('#file-list tr[data-path="' + CSS.escape(path) + '"]').forEach(tr => {
                tr.classList.toggle('selected', selected);
                tr.querySelector('.select-checkbox').checked = selected;
            });
            updateSelectionUI();
        }

        function toggleSelectAll() {
            const checked = document.getElementById('select-all').checked;
            currentFilesData.forEach(f => {
                if (checked) selectedFiles.add(f.path);
                else selectedFiles.delete(f.path);
            });
            renderFiles(currentFilesData);
            updateSelectionUI();
        }

        function clearSelection() {
            selectedFiles.clear();
            document.querySelectorAll('#file-list tr').forEach(tr => {
                tr.classList.remove('selected');
                const cb = tr.querySelector('.select-checkbox');
                if (cb) cb.checked = false;
            });
            updateSelectionUI();
        }

        function updateSelectionUI() {
            const count = selectedFiles.size;
            selectionToolbar.style.display = count > 0 ? 'flex' : 'none';
            selectionCountText.textContent = count + ' files selected';
        }

        async function deleteSelected() {
            if (!confirm('정말 ' + selectedFiles.size + '개의 항목을 삭제하시겠습니까?')) return;
            loading.style.display = 'flex';
            for (const path of selectedFiles) {
                await fetch('/api/delete', { 
                    method: 'POST', 
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({path})
                });
            }
            loadPath(currentPath, false);
        }

        function copySelected() {
            clipboard = { action: 'copy', paths: Array.from(selectedFiles) };
            alert(selectedFiles.size + '개 항목이 복사되었습니다.');
            updatePasteButton();
        }

        function cutSelected() {
            clipboard = { action: 'move', paths: Array.from(selectedFiles) };
            alert(selectedFiles.size + '개 항목이 잘라내기되었습니다.');
            updatePasteButton();
        }

        function toggleFavorites() {
            const list = document.getElementById('favorites-list');
            const icon = document.getElementById('fav-toggle-icon');
            const isHidden = list.style.display === 'none';
            list.style.display = isHidden ? 'block' : 'none';
            icon.textContent = isHidden ? '▼' : '▶';
        }

        function updateFavButton() {
            favBtn.textContent = favorites.some(f => f.path === currentPath) ? '⭐' : '☆';
        }

        function toggleFavorite() {
            if (currentPath === '/') return; 
            const index = favorites.findIndex(f => f.path === currentPath);
            if (index > -1) favorites.splice(index, 1);
            else {
                const name = currentPath.split('/').pop() || currentPath;
                favorites.push({ name, path: currentPath });
            }
            saveSettingsToServer();
            renderFavorites();
            updateFavButton();
        }

        function renderFavorites() {
            favList.innerHTML = '';
            favorites.forEach((fav, idx) => {
                const div = document.createElement('div');
                div.className = 'nav-item';
                div.innerHTML = '<div class="nav-content"><i>⭐</i> <span>' + fav.name + '</span></div><div class="remove-fav" title="Remove">&times;</div>';
                div.onclick = () => loadPath(fav.path, true);
                div.querySelector('.remove-fav').onclick = (e) => {
                    e.stopPropagation();
                    favorites.splice(idx, 1);
                    saveSettingsToServer();
                    renderFavorites();
                    updateFavButton();
                };
                favList.appendChild(div);
            });
        }

        function showContextMenu(x, y, file) {
            contextMenu.style.display = 'block';
            contextMenu.style.left = Math.min(x, window.innerWidth - contextMenu.offsetWidth) + 'px';
            contextMenu.style.top = Math.min(y, window.innerHeight - contextMenu.offsetHeight) + 'px';
            
            document.getElementById('ctx-download').onclick = () => {
                if(file.isDir) return;
                window.open('/api/download?path=' + encodeURIComponent(file.path), '_blank');
            };
            document.getElementById('ctx-download').style.display = file.isDir ? 'none' : 'block';

            document.getElementById('ctx-rename').onclick = async () => {
                const newName = prompt('Enter new name:', file.name);
                if(newName && newName !== file.name) {
                    const newPath = currentPath === '/' ? '/' + newName : currentPath + (currentPath.endsWith('/') ? '' : '/') + newName;
                    loading.style.display = 'flex';
                    try {
                        await fetch('/api/move', { 
                            method: 'POST', 
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify({ oldPath: file.path, newPath: newPath }) 
                        });
                        loadPath(currentPath, false);
                    } catch(e) { alert('Rename failed'); loading.style.display = 'none'; }
                }
            };

            document.getElementById('ctx-copy').onclick = () => {
                clipboard = { action: 'copy', paths: [file.path] };
                updatePasteButton();
            };
            document.getElementById('ctx-cut').onclick = () => {
                clipboard = { action: 'move', paths: [file.path] };
                updatePasteButton();
            };
            document.getElementById('ctx-delete').onclick = async () => {
                if(confirm('정말 삭제하시겠습니까?\n' + file.name)) {
                    loading.style.display = 'flex';
                    await fetch('/api/delete', { 
                        method: 'POST', 
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({path: file.path})
                    });
                    loadPath(currentPath, false);
                }
            };
        }

        function updatePasteButton() {
            if (clipboard) {
                pasteBtn.style.display = 'flex';
                pasteBtn.textContent = '📋 Paste (' + clipboard.paths.length + ')';
            } else {
                pasteBtn.style.display = 'none';
            }
        }

        async function handlePaste() {
            if(!clipboard) return;
            loading.style.display = 'flex';
            try {
                for (const src of clipboard.paths) {
                    const name = src.split('/').pop();
                    const dest = currentPath === '/' ? '/' + name : currentPath + (currentPath.endsWith('/') ? '' : '/') + name;
                    if (src === dest) continue;
                    
                    if (clipboard.action === 'move') {
                        await fetch('/api/move', { 
                            method: 'POST', 
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify({ oldPath: src, newPath: dest }) 
                        });
                    } else {
                        await fetch('/api/copy', { 
                            method: 'POST', 
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify({ src: src, dst: dest }) 
                        });
                    }
                }
                if (clipboard.action === 'move') clipboard = null;
                updatePasteButton();
                loadPath(currentPath, false);
            } catch(e) {
                alert('작업 실패: ' + e);
                loading.style.display = 'none';
            }
        }

        function createFolder() {
            if (document.getElementById('new-folder-input')) return;
            const tr = document.createElement('tr');
            tr.innerHTML = '<td></td><td colspan="4"><div class="name-cell"><span class="icon folder-icon">📁</span><input type="text" id="new-folder-input" placeholder="Name" style="flex: 1; background: var(--bg); color: var(--text); border: 1px solid var(--primary); outline: none; border-radius: 4px; padding: 4px 8px;" /> <button class="action-btn" id="new-folder-save">✅</button><button class="action-btn" id="new-folder-cancel">❌</button></div></td>';
            fileList.insertBefore(tr, fileList.firstChild);
            const input = document.getElementById('new-folder-input');
            input.focus();
            
            const submit = async () => {
                const name = input.value.trim();
                if (!name) { tr.remove(); return; }
                const newPath = currentPath === '/' ? '/' + name : currentPath + (currentPath.endsWith('/') ? '' : '/') + name;
                loading.style.display = 'flex';
                try {
                    await fetch('/api/mkdir', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({ path: newPath }) });
                    loadPath(currentPath, false);
                } catch(e) { alert('폴더 생성 실패'); loading.style.display = 'none'; }
            };
            input.onkeydown = (e) => { if (e.key === 'Enter') submit(); if (e.key === 'Escape') tr.remove(); };
            document.getElementById('new-folder-save').onclick = submit;
            document.getElementById('new-folder-cancel').onclick = () => tr.remove();
        }

        function handleUpload(event) {
            const files = event.target.files || (event.dataTransfer && event.dataTransfer.files);
            if (!files || files.length === 0) return;
            
            document.getElementById('upload-manager').style.display = 'flex';
            
            for (let i = 0; i < files.length; i++) {
                uploadFile(files[i]);
            }
            if(event.target.value) event.target.value = '';
        }

        function uploadFile(file) {
            const uploadId = Math.random().toString(36).substr(2, 9);
            const xhr = new XMLHttpRequest();
            const startTime = Date.now();
            
            activeUploads[uploadId] = { xhr, file, startTime };
            
            const item = document.createElement('div');
            item.className = 'upload-item';
            item.id = 'upload-' + uploadId;
            item.innerHTML = '<div class="upload-item-info">' +
                    '<span class="upload-item-name">' + file.name + '</span>' +
                    '<span class="upload-cancel-btn" onclick="cancelUpload(\'' + uploadId + '\')">✕</span>' +
                '</div>' +
                '<div class="upload-progress-container">' +
                    '<div class="upload-progress-bar" id="pb-' + uploadId + '"></div>' +
                '</div>' +
                '<div class="upload-item-meta">' +
                    '<span id="pct-' + uploadId + '">0%</span>' +
                    '<span id="size-' + uploadId + '">0 / ' + formatBytes(file.size) + '</span>' +
                    '<span id="eta-' + uploadId + '">--:--</span>' +
                '</div>';
            document.getElementById('upload-list').appendChild(item);
            updateUploadSummary();

            const formData = new FormData();
            formData.append('file', file);

            xhr.upload.onprogress = (e) => {
                if (e.lengthComputable) {
                    const pct = Math.round((e.loaded / e.total) * 100);
                    const pb = document.getElementById('pb-' + uploadId);
                    const pctText = document.getElementById('pct-' + uploadId);
                    const sizeText = document.getElementById('size-' + uploadId);
                    const etaText = document.getElementById('eta-' + uploadId);
                    
                    if (pb) pb.style.width = pct + '%';
                    if (pctText) pctText.textContent = pct + '%';
                    if (sizeText) sizeText.textContent = formatBytes(e.loaded) + ' / ' + formatBytes(e.total);
                    
                    const elapsed = (Date.now() - startTime) / 1000;
                    const speed = e.loaded / elapsed;
                    const remaining = (e.total - e.loaded) / speed;
                    if (etaText) etaText.textContent = isFinite(remaining) ? formatTime(remaining) : '--:--';
                }
            };

            xhr.onload = () => {
                const item = document.getElementById('upload-' + uploadId);
                if (item) {
                    item.classList.add('completed');
                    const meta = item.querySelector('.upload-item-meta');
                    if (meta) {
                        meta.innerHTML = '<span style="color: var(--primary); font-weight: bold;">✓ Completed</span>' +
                                       '<span style="cursor: pointer; background: var(--hover); padding: 2px 6px; border-radius: 4px;" onclick="this.closest(\'.upload-item\').remove(); updateUploadSummary();">Clear</span>';
                    }
                    const pb = item.querySelector('.upload-progress-bar');
                    if (pb) pb.style.width = '100%';
                    const cancelBtn = item.querySelector('.upload-cancel-btn');
                    if (cancelBtn) cancelBtn.style.display = 'none';
                }
                delete activeUploads[uploadId];
                updateUploadSummary();
                if (Object.keys(activeUploads).length === 0) {
                    loadPath(currentPath, false);
                }
            };
            
            xhr.onerror = () => alert('Upload failed: ' + file.name);
            
            xhr.open('POST', '/api/upload?path=' + encodeURIComponent(currentPath));
            xhr.send(formData);
        }

        function cancelUpload(id) {
            if (activeUploads[id]) {
                activeUploads[id].xhr.abort();
                delete activeUploads[id];
                document.getElementById('upload-' + id).remove();
                updateUploadSummary();
            }
        }

        function updateUploadSummary() {
            const count = Object.keys(activeUploads).length;
            document.getElementById('upload-summary').textContent = 'Uploads (' + count + ')';
        }

        function toggleUploadManager() {
            const list = document.getElementById('upload-list');
            const icon = document.getElementById('upload-toggle-icon');
            const isHidden = list.style.display === 'none';
            list.style.display = isHidden ? 'block' : 'none';
            icon.textContent = isHidden ? '▼' : '▲';
        }

        function formatTime(seconds) {
            const h = Math.floor(seconds / 3600);
            const m = Math.floor((seconds % 3600) / 60);
            const s = Math.floor(seconds % 60);
            if (h > 0) return h + 'h ' + m + 'm ' + s + 's';
            if (m > 0) return m + 'm ' + s + 's';
            return s + 's';
        }

        dropZone.addEventListener('dragover', (e) => { 
            if (e.dataTransfer.types && Array.from(e.dataTransfer.types).includes('Files')) {
                e.preventDefault(); 
                dragOverlay.style.display = 'flex'; 
            }
        });
        dropZone.addEventListener('dragleave', (e) => { e.preventDefault(); dragOverlay.style.display = 'none'; });
        dropZone.addEventListener('drop', (e) => {
            if (e.dataTransfer.types && Array.from(e.dataTransfer.types).includes('Files')) {
                e.preventDefault();
                dragOverlay.style.display = 'none';
                handleUpload(e);
            }
        });

        const modal = document.getElementById('file-modal');
        function showModal(file) {
            document.getElementById('modal-filename').textContent = file.name;
            const ext = file.name.split('.').pop().toLowerCase();
            document.getElementById('modal-type').textContent = ext.toUpperCase() + ' File';
            document.getElementById('modal-size').textContent = file.size;
            document.getElementById('modal-time').textContent = file.modTime;
            document.getElementById('modal-path').textContent = file.path;
            
            const previewDiv = document.getElementById('modal-preview');
            previewDiv.innerHTML = '';
            const imgExts = ['jpg','jpeg','png','gif','webp','bmp','svg'];
            const vidExts = ['mp4','webm','ogg','ts'];
            const audExts = ['mp3','wav','ogg','flac'];
            const txtExts = ['txt','md','json','js','go','csv','html','css', 'log', 'sh', 'py', 'xml'];
            const fileUrl = '/api/download?path=' + encodeURIComponent(file.path);
            
            const saveBtn = document.getElementById('modal-save-btn');
            if(saveBtn) saveBtn.style.display = 'none';

            if (imgExts.includes(ext)) {
                previewDiv.innerHTML = '<img src="' + fileUrl + '" style="max-width: 100%; max-height: 250px; object-fit: contain; border-radius: 4px;">';
            } else if (vidExts.includes(ext)) {
                previewDiv.innerHTML = '<video src="' + fileUrl + '" controls style="max-width: 100%; max-height: 250px; border-radius: 4px;"></video>';
            } else if (audExts.includes(ext)) {
                previewDiv.innerHTML = '<audio src="' + fileUrl + '" controls style="width: 100%;"></audio>';
            } else if (txtExts.includes(ext)) {
                previewDiv.innerHTML = '<textarea id="text-editor" style="width:100%; height: 250px; resize: none; background: var(--surface); color: var(--text); border: 1px solid var(--border); border-radius: 4px; padding: 8px; font-family: monospace; box-sizing: border-box;"></textarea>';
                fetch(fileUrl).then(res => res.text()).then(text => {
                    const editor = document.getElementById('text-editor');
                    if (editor) editor.value = text;
                });
                if(saveBtn) {
                    saveBtn.style.display = 'block';
                    saveBtn.onclick = async () => {
                        const content = document.getElementById('text-editor').value;
                        saveBtn.textContent = 'Saving...';
                        try {
                            await fetch('/api/save', { method: 'POST', headers: {'Content-Type': 'application/json'}, body: JSON.stringify({path: file.path, content}) });
                            saveBtn.textContent = 'Saved!';
                            loadPath(currentPath, false);
                            setTimeout(closeModal, 700);
                        } catch(e) { alert('저장 실패'); saveBtn.textContent = 'Save'; }
                    };
                }
            } else {
                previewDiv.innerHTML = '<div style="font-size: 4rem; margin: 20px 0;">📄</div>';
            }

            document.getElementById('modal-download-btn').onclick = () => {
                window.open('/api/download?path=' + encodeURIComponent(file.path), '_blank');
                closeModal();
            };
            modal.style.display = 'flex';
        }
        function closeModal() { modal.style.display = 'none'; }
        
        function checkPing() {
            const start = Date.now();
            fetch('/api/ping')
                .then(() => {
                    const latency = Date.now() - start;
                    document.getElementById('ping-display').textContent = 'Ping: ' + latency + ' ms';
                })
                .catch(() => {
                    document.getElementById('ping-display').textContent = 'Ping: Error';
                });
        }
        setInterval(checkPing, 5000);
        checkPing();

        loadSettings();
        loadPath(new URLSearchParams(window.location.search).get('path') || '/', false);
    </script>
</body>
</html>`

func apiListHandler(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	if reqPath == "" {
		reqPath = "/"
	}
	
	fullPath := filepath.Join(rootDir, filepath.Clean(reqPath))
	
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var files []FileInfo
	for _, entry := range entries {
		info, _ := entry.Info()
		size := info.Size()
		if entry.IsDir() {
			size = getDirSize(filepath.Join(fullPath, entry.Name()))
		}
		files = append(files, FileInfo{
			Name:       entry.Name(),
			IsDir:      entry.IsDir(),
			Size:       formatSize(size),
			SizeBytes:  size,
			ModTime:    info.ModTime().Format("2006-01-02 15:04"),
			ModTimeRaw: info.ModTime().Unix(),
			Path:       filepath.Join(reqPath, entry.Name()),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"currentPath": reqPath,
		"files":       files,
	})
}

func apiDownloadHandler(w http.ResponseWriter, r *http.Request) {
	reqPath := r.URL.Query().Get("path")
	fullPath := filepath.Join(rootDir, filepath.Clean(reqPath))
	http.ServeFile(w, r, fullPath)
}

func apiUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	reqPath := r.URL.Query().Get("path")
	fullPath := filepath.Join(rootDir, filepath.Clean(reqPath))

	if err := r.ParseMultipartForm(100 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	files := r.MultipartForm.File["file"]
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		dstPath := filepath.Join(fullPath, fileHeader.Filename)
		dst, err := os.Create(dstPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}

func apiDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var data struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(rootDir, filepath.Clean(data.Path))
	if err := os.RemoveAll(fullPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func apiSaveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var data struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(rootDir, filepath.Clean(data.Path))
	if err := os.WriteFile(fullPath, []byte(data.Content), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}


var settingsPath string

func init() {
	execPath, err := os.Executable()
	if err != nil {
		settingsPath = "mistvault_settings.json"
	} else {
		settingsPath = filepath.Join(filepath.Dir(execPath), "mistvault_settings.json")
	}
}

func apiSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			if os.IsNotExist(err) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"theme":"","favorites":[]}`))
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	} else if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := os.WriteFile(settingsPath, body, 0644); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func apiMkdirHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var data struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fullPath := filepath.Join(rootDir, filepath.Clean(data.Path))
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func apiRenameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var data struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fullOldPath := filepath.Join(rootDir, filepath.Clean(data.OldPath))
	fullNewPath := filepath.Join(rootDir, filepath.Clean(data.NewPath))
	if err := os.Rename(fullOldPath, fullNewPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil { return err }
	return out.Close()
}

func copyDir(src string, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil { return err }
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil { return err }

	entries, err := os.ReadDir(src)
	if err != nil { return err }

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err = copyDir(srcPath, dstPath); err != nil { return err }
		} else {
			if err = copyFile(srcPath, dstPath); err != nil { return err }
		}
	}
	return nil
}

func apiCopyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var data struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fullSrc := filepath.Join(rootDir, filepath.Clean(data.Src))
	fullDst := filepath.Join(rootDir, filepath.Clean(data.Dst))

	info, err := os.Stat(fullSrc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.IsDir() {
		err = copyDir(fullSrc, fullDst)
	} else {
		err = copyFile(fullSrc, fullDst)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

var (
	sizeCache      = make(map[string]int64)
	sizeCacheTime  = make(map[string]time.Time)
	sizeCacheMutex sync.RWMutex
)

func getDirSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}

	sizeCacheMutex.RLock()
	cachedSize, exists := sizeCache[path]
	cachedTime, timeExists := sizeCacheTime[path]
	sizeCacheMutex.RUnlock()

	if exists && timeExists && info.ModTime().Equal(cachedTime) {
		return cachedSize
	}

	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	sizeCacheMutex.Lock()
	sizeCache[path] = size
	sizeCacheTime[path] = info.ModTime()
	sizeCacheMutex.Unlock()

	return size
}

func apiStorageHandler(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var storage []StorageInfo
	
	// Add root drive
	var rootStat syscall.Statfs_t
	if err := syscall.Statfs("/", &rootStat); err == nil {
		total := rootStat.Blocks * uint64(rootStat.Bsize)
		free := rootStat.Bfree * uint64(rootStat.Bsize)
		storage = append(storage, StorageInfo{
			Name:  "Main Drive",
			Path:  "/",
			Total: total,
			Free:  free,
			Used:  total - free,
		})
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(rootDir, entry.Name())
		var stat syscall.Statfs_t
		err := syscall.Statfs(path, &stat)
		if err != nil {
			continue
		}

		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bfree * uint64(stat.Bsize)
		if total == 0 { continue }

		storage = append(storage, StorageInfo{
			Name:  entry.Name(),
			Path:  path,
			Total: total,
			Free:  free,
			Used:  total - free,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(storage)
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		fmt.Fprint(w, htmlTemplate)
	})

	http.HandleFunc("/api/list", apiListHandler)
	http.HandleFunc("/api/download", apiDownloadHandler)
	http.HandleFunc("/api/upload", apiUploadHandler)
	http.HandleFunc("/api/delete", apiDeleteHandler)
	http.HandleFunc("/api/move", apiRenameHandler)
	http.HandleFunc("/api/copy", apiCopyHandler)
	http.HandleFunc("/api/mkdir", apiMkdirHandler)
	http.HandleFunc("/api/settings", apiSettingsHandler)
	http.HandleFunc("/api/save", apiSaveHandler)
	http.HandleFunc("/api/storage", apiStorageHandler)
	http.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("👻 MistVault Explorer starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
