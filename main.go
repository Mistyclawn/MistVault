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
        
        body { font-family: "Segoe UI", Roboto, "Helvetica Neue", sans-serif; background: var(--bg); color: var(--text); padding: 0; margin: 0; overflow: hidden; transition: background 0.3s, color 0.3s; }
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
        
        table { width: 100%; border-collapse: collapse; margin-top: 10px; table-layout: fixed; }
        thead { position: sticky; top: 0; background: var(--bg); z-index: 10; box-shadow: 0 1px 0 var(--border); }
        th { text-align: left; padding: 12px 16px; color: var(--text-dim); font-size: 0.875rem; font-weight: 500; cursor: pointer; user-select: none; }
        td { padding: 12px 16px; border-bottom: 1px solid var(--border); font-size: 0.875rem; cursor: default; color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
        tr:hover td { background: var(--hover); }
        tr.selected td { background: var(--selected); }
        
        .name-cell { display: flex; align-items: center; overflow: hidden; }
        .name-text { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
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
            </div>
            
            <div class="content-area">
                <div class="toolbar">
                    <div style="display: flex; flex-grow: 1; align-items: center; border: 1px solid var(--border); border-radius: 4px; padding: 4px 8px; background: var(--bg);">
                        <div id="breadcrumb" class="breadcrumb" style="flex-grow: 1; display: flex;" onclick="editPath()"></div>
                        <input type="text" id="path-input" style="display: none; width: 100%; background: transparent; border: none; color: var(--text); outline: none; font-size: 1rem;" onkeydown="handlePathInput(event)">
                        <button class="action-btn" onclick="copyCurrentPath()" title="Copy Path" style="margin-left: 8px;">📋</button>
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

        function toggleSidebar() {
            const sidebar = document.querySelector('.sidebar');
            if (sidebar.style.display === 'none') {
                sidebar.style.display = 'flex';
            } else {
                sidebar.style.display = 'none';
            }
        }

        let currentSort = { col: 'name', asc: true };
        let currentFilesData = [];

        function sortFiles(col) {
            if (currentSort.col === col) {
                currentSort.asc = !currentSort.asc;
            } else {
                currentSort.col = col;
                currentSort.asc = true;
            }
            document.querySelectorAll('th').forEach(th => th.textContent = th.textContent.replace(' ↑', '').replace(' ↓', ''));
            const th = document.getElementById('th-' + col);
            if (th) th.textContent += (currentSort.asc ? ' ↑' : ' ↓');
            renderFiles(currentFilesData);
        }
        let clipboard = null;
        let favorites = [];
        try {
            favorites = JSON.parse(localStorage.getItem('mistvault_favorites') || '[]');
            if (!Array.isArray(favorites)) favorites = [];
        } catch(e) {
            favorites = [];
            localStorage.setItem('mistvault_favorites', '[]');
        }

        const fileList = document.getElementById('file-list');
        const breadcrumb = document.getElementById('breadcrumb');
        const loading = document.getElementById('loading');
        const favList = document.getElementById('favorites-list');
        const favBtn = document.getElementById('fav-btn');
        const pasteBtn = document.getElementById('paste-btn');
        const contextMenu = document.getElementById('context-menu');
        const dropZone = document.getElementById('drop-zone');
        const dragOverlay = document.getElementById('drag-overlay');

        // Theme initialization
        const savedTheme = localStorage.getItem('mistvault_theme');
        if (savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)) {
            document.body.classList.add('dark-theme');
            document.getElementById('theme-btn').textContent = '☀️ Light';
        }

        function toggleTheme() {
            const isDark = document.body.classList.toggle('dark-theme');
            localStorage.setItem('mistvault_theme', isDark ? 'dark' : 'light');
            document.getElementById('theme-btn').textContent = isDark ? '☀️ Light' : '🌙 Dark';
        }

        renderFavorites();
        updatePasteButton();
        
        window.addEventListener('popstate', (event) => {
            const path = new URLSearchParams(window.location.search).get('path') || '/';
            loadPath(path, false);
        });

        
        function editPath() {
            const bc = document.getElementById('breadcrumb');
            const input = document.getElementById('path-input');
            bc.style.display = 'none';
            input.style.display = 'block';
            input.value = currentPath;
            input.focus();
        }
        function handlePathInput(e) {
            if (e.key === 'Enter') {
                const input = document.getElementById('path-input');
                loadPath(input.value, true);
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
            if (input && input.style.display === 'block' && e.target !== input && !e.target.closest('#breadcrumb')) {
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
            try {
                const response = await fetch('/api/list?path=' + encodeURIComponent(path));
                if (!response.ok) throw new Error('Network response was not ok');
                const data = await response.json();
                
                currentPath = data.currentPath || '/';
                renderBreadcrumb(currentPath);
                currentFilesData = data.files || []; renderFiles(currentFilesData);
                updateFavButton();
                
                if (push) {
                    const newUrl = currentPath === '/' ? window.location.pathname : window.location.pathname + '?path=' + encodeURIComponent(currentPath);
                    window.history.pushState({ path: currentPath }, '', newUrl);
                }
            } catch (err) {
                console.error('Failed to load path:', err);
                fileList.innerHTML = '<tr><td colspan="4" class="empty-state">경로를 로드할 수 없습니다.</td></tr>';
            } finally {
                loading.style.display = 'none';
            }
        }

        function renderBreadcrumb(path) {
            const parts = path.split('/').filter(p => p);
            breadcrumb.innerHTML = '';
            
            const rootSpan = document.createElement('span');
            rootSpan.textContent = 'MistVault';
            rootSpan.onclick = () => loadPath('/', true);
            breadcrumb.appendChild(rootSpan);

            let accumulatedPath = '';
            parts.forEach(part => {
                accumulatedPath += '/' + part;
                const thisPath = accumulatedPath;
                const sepSpan = document.createElement('span');
                sepSpan.className = 'separator';
                sepSpan.textContent = '›';
                breadcrumb.appendChild(sepSpan);
                const partSpan = document.createElement('span');
                partSpan.textContent = part;
                partSpan.onclick = () => loadPath(thisPath, true);
                breadcrumb.appendChild(partSpan);
            });
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
            if (currentPath !== '/' && currentPath !== '') {
                const parentPath = currentPath.substring(0, currentPath.lastIndexOf('/')) || '/';
                const tr = document.createElement('tr');
                tr.innerHTML = '<td colspan="4"><div class="name-cell"><span class="icon folder-icon">📁</span><span class="name-text">..</span></div></td>';
                tr.ondblclick = () => loadPath(parentPath, true);
                tr.onclick = () => selectRow(tr);
                fileList.appendChild(tr);
            }

            if (files.length === 0) {
                const tr = document.createElement('tr');
                tr.innerHTML = '<td colspan="4" class="empty-state">이 폴더는 비어 있습니다.</td>';
                fileList.appendChild(tr);
                return;
            }

            files.forEach(file => {
                const tr = document.createElement('tr');
                
                const tdName = document.createElement('td');
                const nameDiv = document.createElement('div');
                nameDiv.className = 'name-cell';
                const iconSpan = document.createElement('span');
                iconSpan.className = 'icon ' + (file.isDir ? 'folder-icon' : 'file-icon');
                iconSpan.textContent = file.isDir ? '📁' : '📄';
                const nameSpan = document.createElement('span');
                nameSpan.className = 'name-text';
                nameSpan.textContent = file.name;
                nameSpan.title = file.name; // Tooltip for long names
                nameDiv.appendChild(iconSpan);
                nameDiv.appendChild(nameSpan);
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
                
                tr.appendChild(tdName);
                tr.appendChild(tdSize);
                tr.appendChild(tdTime);
                tr.appendChild(tdActions);
                
                tr.ondblclick = () => {
                    if (file.isDir) {
                        loadPath(file.path, true);
                    } else {
                        showModal(file);
                    }
                };
                tr.onclick = () => selectRow(tr);
                tr.oncontextmenu = (e) => {
                    e.preventDefault();
                    selectRow(tr);
                    showContextMenu(e.clientX, e.clientY, file);
                };

                fileList.appendChild(tr);
            });
        }

        function selectRow(tr) {
            document.querySelectorAll('#file-list tr').forEach(r => r.classList.remove('selected'));
            tr.classList.add('selected');
        }

        
        function toggleFavorites() {
            const list = document.getElementById('favorites-list');
            const icon = document.getElementById('fav-toggle-icon');
            if (list.style.display === 'none') {
                list.style.display = 'block';
                icon.textContent = '▼';
            } else {
                list.style.display = 'none';
                icon.textContent = '▶';
            }
        }

        // Favorites
        function updateFavButton() {
            favBtn.textContent = favorites.some(f => f.path === currentPath) ? '⭐' : '☆';
        }
        function toggleFavorite() {
            if (currentPath === '/') return; 
            const index = favorites.findIndex(f => f.path === currentPath);
            if (index > -1) {
                favorites.splice(index, 1);
            } else {
                const name = currentPath.split('/').pop() || currentPath;
                favorites.push({ name, path: currentPath });
            }
            localStorage.setItem('mistvault_favorites', JSON.stringify(favorites));
            renderFavorites();
            updateFavButton();
        }
        function renderFavorites() {
            favList.innerHTML = '';
            favorites.forEach((fav, idx) => {
                const div = document.createElement('div');
                div.className = 'nav-item';
                div.innerHTML = '<div class="nav-content"><i>⭐</i> <span style="white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">' + fav.name + '</span></div><div class="remove-fav" title="Remove">&times;</div>';
                div.onclick = () => loadPath(fav.path, true);
                div.querySelector('.remove-fav').onclick = (e) => {
                    e.stopPropagation();
                    favorites.splice(idx, 1);
                    localStorage.setItem('mistvault_favorites', JSON.stringify(favorites));
                    renderFavorites();
                    updateFavButton();
                };
                favList.appendChild(div);
            });
        }

        // Context Menu
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
                    const newPath = currentPath === '/' ? '/' + newName : currentPath + '/' + newName;
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
                clipboard = { action: 'copy', path: file.path, name: file.name };
                updatePasteButton();
            };
            document.getElementById('ctx-cut').onclick = () => {
                clipboard = { action: 'move', path: file.path, name: file.name };
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
                pasteBtn.textContent = '📋 Paste (' + clipboard.name + ')';
            } else {
                pasteBtn.style.display = 'none';
            }
        }

        async function handlePaste() {
            if(!clipboard) return;
            const destPath = currentPath === '/' ? '/' + clipboard.name : currentPath + '/' + clipboard.name;
            if (destPath === clipboard.path) return;

            loading.style.display = 'flex';
            try {
                if (clipboard.action === 'move') {
                    await fetch('/api/move', { 
                        method: 'POST', 
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({ oldPath: clipboard.path, newPath: destPath }) 
                    });
                    clipboard = null;
                } else {
                    await fetch('/api/copy', { 
                        method: 'POST', 
                        headers: {'Content-Type': 'application/json'},
                        body: JSON.stringify({ src: clipboard.path, dst: destPath }) 
                    });
                }
                updatePasteButton();
                loadPath(currentPath, false);
            } catch(e) {
                alert('작업 실패: ' + e);
                loading.style.display = 'none';
            }
        }

        async function createFolder() {
            const name = prompt("새 폴더 이름:");
            if (!name) return;
            const newPath = currentPath === '/' ? '/' + name : currentPath + '/' + name;
            loading.style.display = 'flex';
            try {
                await fetch('/api/mkdir', { 
                    method: 'POST', 
                    headers: {'Content-Type': 'application/json'},
                    body: JSON.stringify({ path: newPath }) 
                });
                loadPath(currentPath, false);
            } catch(e) {
                alert('폴더 생성 실패');
                loading.style.display = 'none';
            }
        }

        // Upload
        async function handleUpload(event) {
            const files = event.target.files || (event.dataTransfer && event.dataTransfer.files);
            if (!files || files.length === 0) return;
            
            const formData = new FormData();
            for (let i = 0; i < files.length; i++) {
                formData.append('file', files[i]);
            }
            
            loading.style.display = 'flex';
            try {
                await fetch('/api/upload?path=' + encodeURIComponent(currentPath), {
                    method: 'POST',
                    body: formData
                });
                loadPath(currentPath, false);
            } catch(e) {
                alert('업로드 실패');
            } finally {
                loading.style.display = 'none';
                if(event.target.value) event.target.value = '';
            }
        }

        // Drag & Drop
        dropZone.addEventListener('dragover', (e) => { e.preventDefault(); dragOverlay.style.display = 'flex'; });
        dropZone.addEventListener('dragleave', (e) => { e.preventDefault(); dragOverlay.style.display = 'none'; });
        dropZone.addEventListener('drop', (e) => {
            e.preventDefault();
            dragOverlay.style.display = 'none';
            handleUpload(e);
        });

        // Modal
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
            const fileUrl = '/api/download?path=' + encodeURIComponent(file.path);
            
            const txtExts = ['txt','md','json','js','go','csv','html','css', 'log', 'sh', 'py', 'xml'];
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
                        const saveBtnOrig = saveBtn.textContent;
                        saveBtn.textContent = 'Saving...';
                        try {
                            const res = await fetch('/api/save', {
                                method: 'POST',
                                headers: {'Content-Type': 'application/json'},
                                body: JSON.stringify({path: file.path, content: content})
                            });
                            if (!res.ok) throw new Error('Save failed');
                            saveBtn.textContent = 'Saved!';
                            loadPath(currentPath, false);
                            setTimeout(() => { closeModal(); }, 700);
                        } catch(e) {
                            alert('저장 실패: ' + e);
                            saveBtn.textContent = saveBtnOrig;
                        }
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
		files = append(files, FileInfo{
			Name:       entry.Name(),
			IsDir:      entry.IsDir(),
			Size:       formatSize(info.Size()),
			SizeBytes:  info.Size(),
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
	http.HandleFunc("/api/save", apiSaveHandler)

	log.Printf("👻 MistVault Explorer starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
