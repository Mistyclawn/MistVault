package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FileInfo 구조체
type FileInfo struct {
	Name    string `json:"name"`
	Size    string `json:"size"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
	Path    string `json:"path"`
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

const htmlTemplate = `
<!DOCTYPE html>
<html lang="ko">
<head>
    <meta charset="UTF-8">
    <title>MistVault Explorer</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root { 
            --bg: #f8f9fa; 
            --surface: #ffffff; 
            --primary: #1a73e8; 
            --secondary: #5f6368; 
            --text: #202124; 
            --text-dim: #5f6368;
            --border: #dadce0;
            --hover: #f1f3f4;
            --selected: #e8f0fe;
        }
        @media (prefers-color-scheme: dark) {
            :root {
                --bg: #202124; 
                --surface: #2d2e30; 
                --primary: #8ab4f8; 
                --secondary: #9aa0a6; 
                --text: #e8eaed; 
                --text-dim: #9aa0a6;
                --border: #5f6368;
                --hover: #3c4043;
                --selected: #3b4252;
            }
        }

        body { font-family: "Segoe UI", Roboto, "Helvetica Neue", sans-serif; background: var(--bg); color: var(--text); padding: 0; margin: 0; overflow: hidden; }
        .app-container { display: flex; height: 100vh; flex-direction: column; }
        
        header { background: var(--surface); padding: 10px 24px; border-bottom: 1px solid var(--border); display: flex; align-items: center; justify-content: space-between; height: 48px;}
        .logo { display: flex; align-items: center; font-size: 1.25rem; color: var(--text); font-weight: 500; }
        .logo span { margin-right: 12px; font-size: 1.5rem; }

        .main-area { display: flex; flex-grow: 1; overflow: hidden; }
        
        .sidebar { width: 240px; background: var(--surface); border-right: 1px solid var(--border); padding: 16px 0; display: flex; flex-direction: column; }
        .nav-item { padding: 10px 24px; display: flex; align-items: center; color: var(--text); text-decoration: none; font-weight: 500; font-size: 0.875rem; cursor: pointer;}
        .nav-item:hover { background: var(--hover); }
        .nav-item.active { background: var(--selected); color: var(--primary); border-radius: 0 24px 24px 0; margin-right: 16px; }
        .nav-item i { margin-right: 16px; font-size: 1.2rem; font-style: normal; }

        .content-area { flex-grow: 1; display: flex; flex-direction: column; background: var(--bg); overflow: hidden; }
        
        .toolbar { padding: 12px 24px; display: flex; align-items: center; border-bottom: 1px solid var(--border); background: var(--surface); }
        .breadcrumb { font-size: 1.1rem; color: var(--text); display: flex; align-items: center; white-space: nowrap; overflow-x: auto; scrollbar-width: none; }
        .breadcrumb::-webkit-scrollbar { display: none; }
        .breadcrumb span { cursor: pointer; border-radius: 4px; padding: 4px 8px; transition: background 0.2s; }
        .breadcrumb span:hover { background: var(--hover); }
        .breadcrumb .separator { margin: 0 4px; color: var(--text-dim); cursor: default; }
        .breadcrumb .separator:hover { background: transparent; }

        .file-list-container { flex-grow: 1; overflow-y: auto; padding: 0 24px; }
        
        table { width: 100%; border-collapse: collapse; margin-top: 10px; }
        thead { position: sticky; top: 0; background: var(--bg); z-index: 10; box-shadow: 0 1px 0 var(--border); }
        th { text-align: left; padding: 12px 16px; color: var(--text-dim); font-size: 0.875rem; font-weight: 500; cursor: pointer; user-select: none; }
        th:hover { background: var(--hover); }
        td { padding: 12px 16px; border-bottom: 1px solid var(--border); font-size: 0.875rem; cursor: default; white-space: nowrap; color: var(--text); }
        tr:hover td { background: var(--hover); }
        tr.selected td { background: var(--selected); }
        
        .name-cell { display: flex; align-items: center; }
        .icon { width: 24px; font-size: 1.2rem; display: inline-flex; align-items: center; justify-content: center; margin-right: 16px; }
        .folder-icon { color: var(--primary); }
        .file-icon { color: var(--secondary); }
        
        .col-size { width: 120px; text-align: right; }
        .col-time { width: 160px; text-align: right; }

        ::-webkit-scrollbar { width: 8px; height: 8px; }
        ::-webkit-scrollbar-track { background: transparent; }
        ::-webkit-scrollbar-thumb { background: var(--border); border-radius: 4px; }
        ::-webkit-scrollbar-thumb:hover { background: var(--text-dim); }

        .loading-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.2); display: none; align-items: center; justify-content: center; z-index: 100; backdrop-filter: blur(2px); }
        .spinner { width: 40px; height: 40px; border: 4px solid var(--border); border-top-color: var(--primary); border-radius: 50%; animation: spin 1s linear infinite; }
        @keyframes spin { to { transform: rotate(360deg); } }
        
        .empty-state { padding: 40px; text-align: center; color: var(--text-dim); }
    </style>
</head>
<body>
    <div class="app-container">
        <header>
            <div class="logo"><span>☁️</span> MistVault Explorer</div>
        </header>
        
        <div class="main-area">
            <div class="sidebar">
                <div class="nav-item active" onclick="loadPath('/', true)">
                    <i>🏠</i> Home
                </div>
            </div>
            
            <div class="content-area">
                <div class="toolbar">
                    <div id="breadcrumb" class="breadcrumb"></div>
                </div>

                <div class="file-list-container">
                    <table id="file-table">
                        <thead>
                            <tr>
                                <th>Name</th>
                                <th class="col-size">Size</th>
                                <th class="col-time">Modified</th>
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

    <script>
        let currentPath = '/';
        const fileList = document.getElementById('file-list');
        const breadcrumb = document.getElementById('breadcrumb');
        const loading = document.getElementById('loading');

        window.addEventListener('popstate', (event) => {
            const path = new URLSearchParams(window.location.search).get('path') || '/';
            loadPath(path, false);
        });

        async function loadPath(path, push = true) {
            loading.style.display = 'flex';
            try {
                const response = await fetch('/api/list?path=' + encodeURIComponent(path));
                if (!response.ok) throw new Error('Network response was not ok');
                const data = await response.json();
                
                currentPath = data.currentPath || '/';
                renderBreadcrumb(currentPath);
                renderFiles(data.files || []);
                
                if (push) {
                    const newUrl = currentPath === '/' ? window.location.pathname : window.location.pathname + '?path=' + encodeURIComponent(currentPath);
                    window.history.pushState({ path: currentPath }, '', newUrl);
                }
            } catch (err) {
                console.error('Failed to load path:', err);
                fileList.innerHTML = '<tr><td colspan="3" class="empty-state">경로를 로드할 수 없습니다.</td></tr>';
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

        function renderFiles(files) {
            fileList.innerHTML = '';
            
            if (currentPath !== '/' && currentPath !== '') {
                const parentPath = currentPath.substring(0, currentPath.lastIndexOf('/')) || '/';
                const tr = document.createElement('tr');
                const td = document.createElement('td');
                td.colSpan = 3;
                
                const nameDiv = document.createElement('div');
                nameDiv.className = 'name-cell';
                
                const iconSpan = document.createElement('span');
                iconSpan.className = 'icon folder-icon';
                iconSpan.textContent = '📁';
                
                const nameSpan = document.createElement('span');
                nameSpan.textContent = '..';
                
                nameDiv.appendChild(iconSpan);
                nameDiv.appendChild(nameSpan);
                td.appendChild(nameDiv);
                tr.appendChild(td);
                
                tr.ondblclick = () => loadPath(parentPath, true);
                tr.onclick = () => selectRow(tr);
                fileList.appendChild(tr);
            }

            if (files.length === 0) {
                const tr = document.createElement('tr');
                tr.innerHTML = '<td colspan="3" class="empty-state">이 폴더는 비어 있습니다.</td>';
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
                nameSpan.textContent = file.name;
                
                nameDiv.appendChild(iconSpan);
                nameDiv.appendChild(nameSpan);
                tdName.appendChild(nameDiv);
                
                const tdSize = document.createElement('td');
                tdSize.className = 'col-size';
                tdSize.textContent = file.size;
                
                const tdTime = document.createElement('td');
                tdTime.className = 'col-time';
                tdTime.textContent = file.modTime;
                
                tr.appendChild(tdName);
                tr.appendChild(tdSize);
                tr.appendChild(tdTime);
                
                tr.ondblclick = () => {
                    if (file.isDir) {
                        loadPath(file.path, true);
                    } else {
                        window.open('/api/download?path=' + encodeURIComponent(file.path), '_blank');
                    }
                };

                tr.onclick = () => selectRow(tr);

                fileList.appendChild(tr);
            });
        }

        function selectRow(tr) {
            document.querySelectorAll('#file-list tr').forEach(r => r.classList.remove('selected'));
            tr.classList.add('selected');
        }

        const initialPath = new URLSearchParams(window.location.search).get('path') || '/';
        loadPath(initialPath, false);
    </script>
</body>
</html>
`

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
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			Size:    formatSize(info.Size()),
			ModTime: info.ModTime().Format("2006-01-02 15:04"),
			Path:    filepath.Join(reqPath, entry.Name()),
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

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, htmlTemplate)
	})

	http.HandleFunc("/api/list", apiListHandler)
	http.HandleFunc("/api/download", apiDownloadHandler)

	log.Printf("👻 MistVault Explorer starting on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
