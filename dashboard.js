import { execSync, spawn } from 'child_process';
import http from 'http';
import path from 'path';
import fs from 'fs';

/**
 * MistVault Gateway Dashboard
 * M4 Mac mini optimized, native Node.js
 */

const PORT = 8088;
const STORAGE_SERVER_PATH = '/Volumes/ROGALLY/github/MistVault/mistvault_storage_server';
const LOLMANAGER_WEB_ROOT = '/Volumes/ROGALLY/github/LOLManager/game_client/fresh_start_web';

const MIME_TYPES = {
    '.html': 'text/html; charset=utf-8',
    '.js': 'application/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.json': 'application/json; charset=utf-8',
    '.png': 'image/png',
    '.svg': 'image/svg+xml; charset=utf-8'
};

const serveLolManager = (req, res) => {
    const requestUrl = new URL(req.url, 'http://localhost');
    let relPath = decodeURIComponent(requestUrl.pathname.replace(/^\/lolmanager\/?/, ''));
    if (!relPath || relPath.endsWith('/')) relPath += 'index.html';

    const safePath = path.normalize(relPath).replace(/^(\.\.(\/|\\|$))+/, '');
    const filePath = path.join(LOLMANAGER_WEB_ROOT, safePath);
    const allowed = [
        path.join(LOLMANAGER_WEB_ROOT, 'index.html'),
        path.join(LOLMANAGER_WEB_ROOT, 'styles.css'),
        path.join(LOLMANAGER_WEB_ROOT, 'main.js'),
        path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_index.json'),
        path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_snapshot.json')
    ];

    if (!allowed.includes(filePath) || !fs.existsSync(filePath)) {
        res.writeHead(404, { 'Content-Type': 'text/plain; charset=utf-8' });
        res.end('Not found');
        return;
    }

    const ext = path.extname(filePath);
    res.writeHead(200, {
        'Content-Type': MIME_TYPES[ext] || 'application/octet-stream',
        'Cache-Control': 'no-store'
    });
    fs.createReadStream(filePath).pipe(res);
};

// 스토리지 서버가 죽어있으면 자동으로 살려주는 함수
const ensureStorageServer = () => {
    try {
        execSync('pgrep -f mistvault_storage_server');
    } catch (e) {
        console.log('🚀 Storage server is down. Restarting...');
        if (fs.existsSync(STORAGE_SERVER_PATH)) {
            const out = fs.openSync('/tmp/mistvault_storage_out.log', 'a');
            const err = fs.openSync('/tmp/mistvault_storage_err.log', 'a');
            
            spawn(STORAGE_SERVER_PATH, [], {
                detached: true,
                stdio: [ 'ignore', out, err ]
            }).unref();
        } else {
            console.error('❌ Storage server binary not found at:', STORAGE_SERVER_PATH);
        }
    }
};

// 10초마다 스토리지 서버 상태 체크
setInterval(ensureStorageServer, 10000);
ensureStorageServer();

const getTailscaleIP = () => {
    try {
        const ip = execSync("ifconfig | grep 'inet 100.' | awk '{print $2}'").toString().trim();
        if (ip && ip.startsWith('100.')) return ip;
        const { networkInterfaces } = require('os');
        const nets = networkInterfaces();
        for (const name of Object.keys(nets)) {
            for (const net of nets[name]) {
                if (net.address.startsWith('100.')) return net.address;
            }
        }
        return "localhost";
    } catch (e) {
        return "localhost";
    }
};

const getListeningPorts = () => {
    try {
        const output = execSync("lsof -i -P -n | grep LISTEN").toString();
        const lines = output.trim().split('\n');
        const detectedPorts = lines.map(line => {
            const parts = line.split(/\s+/);
            return parts[8].split(':').pop();
        });

        const coreServices = [
            { name: "🎬 Jellyfin (Media)", port: "8096" },
            { name: "📂 MistVault Storage", port: "8097" },
            { name: "⚔️ Veradom Codex", port: "8099" },
            { name: "🎮 LOLManager Admin", port: "8102" },
            { name: "🏁 LOLManager Fresh Start", port: "8088", path: "/lolmanager/" }
        ];

        return coreServices.map(service => ({
            ...service,
            active: detectedPorts.includes(service.port)
        }));
    } catch (e) {
        return [];
    }
};

const server = http.createServer((req, res) => {
    if (req.url === '/lolmanager' || req.url.startsWith('/lolmanager/')) {
        serveLolManager(req, res);
        return;
    }

    const ports = getListeningPorts();
    const tailscaleIP = getTailscaleIP();
    
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    
    let html = `
    <!DOCTYPE html>
    <html>
    <head>
        <title>👻 MistVault Gateway</title>
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <style>
            body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #0f0f0f; color: #f0f0f0; padding: 20px; line-height: 1.6; }
            .container { max-width: 600px; margin: 0 auto; }
            h1 { color: #bb86fc; text-align: center; font-size: 2.2em; margin-bottom: 5px; }
            .ip-info { text-align: center; color: #666; font-family: monospace; margin-bottom: 30px; font-size: 0.9em; }
            .service-grid { display: grid; gap: 15px; }
            .card { background: #1e1e1e; border-radius: 12px; padding: 20px; border: 1px solid #333; transition: all 0.2s; text-decoration: none; color: inherit; display: flex; justify-content: space-between; align-items: center; }
            .card:hover { transform: translateY(-2px); border-color: #bb86fc; background: #252525; box-shadow: 0 4px 20px rgba(187, 134, 252, 0.1); }
            .status-dot { width: 10px; height: 10px; border-radius: 50%; display: inline-block; margin-right: 10px; }
            .online { background: #03dac6; box-shadow: 0 0 8px #03dac6; }
            .offline { background: #cf6679; }
            .process-name { font-size: 1.1em; font-weight: bold; }
            .port-label { color: #888; font-size: 0.8em; margin-left: 8px; font-family: monospace; }
            .arrow { color: #bb86fc; font-size: 1.2em; opacity: 0.5; }
            .refresh-btn { display: block; width: 100%; text-align: center; padding: 12px; background: #1e1e1e; color: #bb86fc; border-radius: 8px; text-decoration: none; margin-bottom: 20px; font-weight: bold; border: 1px solid #bb86fc; }
            .refresh-btn:hover { background: #bb86fc; color: #000; }
        </style>
    </head>
    <body>
        <div class="container">
            <h1>👻 MistVault Gateway</h1>
            <div class="ip-info">Tailscale: ${tailscaleIP}</div>
            
            <a href="javascript:location.reload()" class="refresh-btn">REFRESH STATUS</a>
            
            <div class="service-grid">
    `;

    ports.forEach(p => {
        const url = `http://${tailscaleIP}:${p.port}${p.path || ''}`;
        html += `
            <a href="${url}" class="card" target="_blank">
                <div>
                    <span class="status-dot ${p.active ? 'online' : 'offline'}"></span>
                    <span class="process-name">${p.name}</span>
                    <span class="port-label">:${p.port}</span>
                </div>
                <div class="arrow">➔</div>
            </a>
        `;
    });

    html += `
            </div>
            <footer style="margin-top: 40px; text-align: center; color: #444; font-size: 0.8em;">
                MistVault &copy; 2026 - Powered by M4 Mac mini
            </footer>
        </div>
    </body>
    </html>
    `;
    
    res.end(html);
});

server.listen(PORT, '0.0.0.0', () => {
    console.log(`MistVault Gateway running at http://0.0.0.0:${PORT}`);
});
