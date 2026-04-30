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
<html>
<head>
    <title>📂 MistVault Storage</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        :root { --bg: #0f0f0f; --surface: #1e1e1e; --primary: #bb86fc; --secondary: #03dac6; --text: #f0f0f0; --text-dim: #888; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--text); padding: 0; margin: 0; overflow: hidden; }
        .app-container { display: flex; flex-direction: column; height: 100vh; }
        
        header { background: var(--surface); padding: 15px 20px; border-bottom: 1px solid #333; display: flex; align-items: center; justify-content: space-between; }
        h1 { margin: 0; font-size: 1.2em; color: var(--secondary); display: flex; align-items: center; }
        h1 span { margin-right: 10px; }
        
        .toolbar { background: #252525; padding: 8px 20px; border-bottom: 1px solid #333; display: flex; align-items: center; }
        .breadcrumb { font-family: monospace; font-size: 0.9em; flex-grow: 1; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
        .breadcrumb span { cursor: pointer; color: var(--primary); }
        .breadcrumb span:hover { text-decoration: underline; }

        .main-content { flex-grow: 1; overflow-y: auto; display: flex; }
        
        table { width: 100%; border-collapse: collapse; position: relative; }
        thead { position: sticky; top: 0; background: #252525; z-index: 10; }
        th { text-align: left; padding: 10px 15px; border-bottom: 1px solid #333; color: var(--text-dim); font-size: 0.85em; font-weight: normal; }
        td { padding: 8px 15px; border-bottom: 1px solid #2a2a2a; font-size: 0.9em; cursor: default; white-space: nowrap; }
        tr:hover { background: #2a2a2a; }
        tr.selected { background: rgba(187, 134, 252, 0.15); }
        
        .icon { width: 24px; display: inline-block; text-align: center; margin-right: 8px; }
        .file-name { color: var(--text); user-select: none; }
        .folder { color: var(--primary); font-weight: 500; }
        
        .col-size { width: 100px; text-align: right; color: var(--text-dim); }
        .col-time { width: 180px; text-align: right; color: var(--text-dim); }

        ::-webkit-scrollbar { width: 8px; }
        ::-webkit-scrollbar-track { background: var(--bg); }
        ::-webkit-scrollbar-thumb { background: #333; border-radius: 4px; }
        ::-webkit-scrollbar-thumb:hover { background: #444; }

        .loading-overlay { position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); display: none; align-items: center; justify-content: center; z-index: 100; }
    </style>
</head>
<body>
    <div class="app-container">
        <header>
            <h1><span>📂</span> MistVault Storage</h1>
            <div style="font-size: 0.8em; color: var(--text-dim);">M4 Native Explorer</div>
        </header>
        
        <div class="toolbar">
            <div id="breadcrumb" class="breadcrumb">/</div>
        </div>

        <div class="main-content">
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

    <div id="loading" class="loading-overlay">Loading...</div>

    <script>
        let currentPath = '/';
        const fileList = document.getElementById('file-list');
        const breadcrumb = document.getElementById('breadcrumb');
        const loading = document.getElementById('loading');

        async function loadPath(path) {
            loading.style.display = 'flex';
            try {
                const response = await fetch('/api/list?path=' + encodeURIComponent(path));
                const data = await response.json();
                
                currentPath = data.currentPath;
                renderBreadcrumb(currentPath);
                renderFiles(data.files);
            } catch (err) {
                console.error('Failed to load path:', err);
                alert('경로를 로드하는 데 실패했어.');
            } finally {
                loading.style.display = 'none';
            }
        }

        function renderBreadcrumb(path) {
            const parts = path.split('/').filter(p => p);
            let html = '<span onclick=\"loadPath(\\'/\\')\">root</span>';
            let accumulatedPath = '';
            
            parts.forEach(part => {
                accumulatedPath += '/' + part;
                const thisPath = accumulatedPath;
                html += ' / <span onclick=\"loadPath(\\'' + thisPath + '\\')\">' + part + '</span>';
            });
            breadcrumb.innerHTML = html;
        }

        function renderFiles(files) {
            fileList.innerHTML = '';
            
            if (currentPath !== '/' && currentPath !== '') {
                const parentPath = currentPath.substring(0, currentPath.lastIndexOf('/')) || '/';
                const tr = document.createElement('tr');
                tr.innerHTML = '<td colspan=\"3\" style=\"color: #666; cursor: pointer; padding: 10px 15px;\">(parent directory)</td>';
                tr.onclick = () => loadPath(parentPath);
                fileList.appendChild(tr);
            }

            files.forEach(file => {
                const tr = document.createElement('tr');
                
                const tdName = document.createElement('td');
                const nameDiv = document.createElement('div');
                nameDiv.className = 'name-cell';
                nameDiv.innerHTML = '<span class=\"icon\">' + (file.isDir ? '📁' : '📄') + '</span>' + 
                                  '<span class=\"file-name ' + (file.isDir ? 'folder' : '') + '\">' + file.name + '</span>';
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
                        loadPath(file.path);
                    } else {
                        window.open('/api/download?path=' + encodeURIComponent(file.path), '_blank');
                    }
                };

                tr.onclick = () => {
                    document.querySelectorAll('tr').forEach(r => r.classList.remove('selected'));
                    tr.classList.add('selected');
                };

                fileList.appendChild(tr);
            });
        }

        loadPath('/');
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
