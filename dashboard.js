import { execSync, execFileSync, spawn } from 'child_process';
import { randomUUID } from 'crypto';
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
const LOLMANAGER_REPO_ROOT = '/Volumes/ROGALLY/github/LOLManager';
const LOLMANAGER_PACKAGE_ROOT = path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_packages');
const LOLMANAGER_PACKAGE_MANIFEST = path.join(LOLMANAGER_PACKAGE_ROOT, 'manifest.json');
const LOLMANAGER_PYTHON = '/usr/local/bin/python3';

const MIME_TYPES = {
    '.html': 'text/html; charset=utf-8',
    '.js': 'application/javascript; charset=utf-8',
    '.css': 'text/css; charset=utf-8',
    '.json': 'application/json; charset=utf-8',
    '.png': 'image/png',
    '.webp': 'image/webp',
    '.jpg': 'image/jpeg',
    '.jpeg': 'image/jpeg',
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
        path.join(LOLMANAGER_WEB_ROOT, 'player-card.js'),
        path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_index.json'),
        path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_snapshot.json')
    ];
    const assetRoots = [
        path.join(LOLMANAGER_WEB_ROOT, 'data', 'assets', 'player_portraits'),
        path.join(LOLMANAGER_WEB_ROOT, 'data', 'assets', 'team_logos')
    ];
    const isAllowedAsset = assetRoots.some((assetRoot) => filePath.startsWith(assetRoot + path.sep)) && ['.png', '.webp', '.jpg', '.jpeg', '.svg'].includes(path.extname(filePath).toLowerCase());
    const isAllowedPackage = filePath.startsWith(LOLMANAGER_PACKAGE_ROOT + path.sep) && path.extname(filePath).toLowerCase() === '.json';

    if ((!allowed.includes(filePath) && !isAllowedAsset && !isAllowedPackage) || !fs.existsSync(filePath)) {
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

const readLolManagerContext = () => ({
    index: JSON.parse(fs.readFileSync(path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_index.json'), 'utf8')),
    snapshot: JSON.parse(fs.readFileSync(path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_snapshot.json'), 'utf8'))
});

const runLolManagerMatchRoom = (action, payload = {}) => {
    const args = ['data_pipeline_v2/scripts/match_room_bridge.py', '--action', action];
    if (payload.saveId) args.push('--save-id', String(payload.saveId));
    if (payload.blueTeamId !== undefined) args.push('--blue-team-id', String(payload.blueTeamId));
    if (payload.redTeamId !== undefined) args.push('--red-team-id', String(payload.redTeamId));
    if (payload.engineMode) args.push('--engine-mode', String(payload.engineMode));
    if (payload.commandId) args.push('--command-id', String(payload.commandId));
    if (payload.name) args.push('--name', String(payload.name));
    if (payload.payload !== undefined) args.push('--payload-json', JSON.stringify(payload.payload || {}));
    const output = execFileSync(LOLMANAGER_PYTHON, args, { cwd: LOLMANAGER_REPO_ROOT, encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 });
    return JSON.parse(output);
};

const normalizedLolManagerName = value => String(value || '').toLowerCase().replace(/[^a-z0-9]+/g, '');

const lolManagerIndexIsFresh = () => {
    const indexPath = path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_index.json');
    const dependencies = [
        path.join(LOLMANAGER_REPO_ROOT, 'data_pipeline_v2', 'lolmanager_v2.db'),
        path.join(LOLMANAGER_REPO_ROOT, 'data_pipeline_v2', 'scripts', 'export_fresh_start_game_index.py')
    ];
    try {
        const indexMtime = fs.statSync(indexPath).mtimeMs;
        return dependencies.every(dependency => !fs.existsSync(dependency) || indexMtime >= fs.statSync(dependency).mtimeMs);
    } catch (_) {
        return false;
    }
};

const exportLolManagerIndex = () => {
    execFileSync('python3', [
        'data_pipeline_v2/scripts/export_fresh_start_game_index.py',
        '--export',
        path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_index.json')
    ], { cwd: LOLMANAGER_REPO_ROOT, stdio: 'pipe' });
    return JSON.parse(fs.readFileSync(path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_index.json'), 'utf8'));
};

const readOrExportLolManagerIndex = () => lolManagerIndexIsFresh()
    ? JSON.parse(fs.readFileSync(path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_index.json'), 'utf8'))
    : exportLolManagerIndex();

const readPackagedLolManagerContext = ({ startYear, team, entryPhase }) => {
    if (!fs.existsSync(LOLMANAGER_PACKAGE_MANIFEST)) return null;
    const year = Number(startYear);
    const phase = String(entryPhase || 'offseason');
    const requested = normalizedLolManagerName(team);
    const manifest = JSON.parse(fs.readFileSync(LOLMANAGER_PACKAGE_MANIFEST, 'utf8'));
    const match = Object.entries(manifest.contexts || {}).find(([, row]) =>
        Number(row.start_year) === year &&
        String(row.entry_phase || 'offseason') === phase &&
        ((row.normalized_aliases || []).includes(requested) || String(row.team_id) === String(team))
    );
    if (!match) return null;
    const [key, row] = match;
    const relative = String(row.path || '').replace(/^\.\//, '').replace(/^data\/fresh_start_packages\//, '');
    const snapshotPath = path.resolve(LOLMANAGER_PACKAGE_ROOT, relative);
    if (!snapshotPath.startsWith(path.resolve(LOLMANAGER_PACKAGE_ROOT) + path.sep) || !fs.existsSync(snapshotPath)) return null;
    return {
        index: readOrExportLolManagerIndex(),
        snapshot: JSON.parse(fs.readFileSync(snapshotPath, 'utf8')),
        package_cache: { hit: true, key, path: row.path, built_at: row.built_at }
    };
};

const sendJson = (res, status, payload) => {
    res.writeHead(status, {
        'Content-Type': 'application/json; charset=utf-8',
        'Cache-Control': 'no-store'
    });
    res.end(JSON.stringify(payload));
};

const freshStartJobs = new Map();

const readProgressFile = (progressFile) => {
    try {
        return JSON.parse(fs.readFileSync(progressFile, 'utf8'));
    } catch (_) {
        return null;
    }
};

const startLolManagerGenerationJob = ({ startYear, team, entryPhase }) => {
    const year = Number(startYear);
    if (!Number.isInteger(year) || year < 2010 || year > 2035) throw new Error('invalid startYear');
    if (!team || String(team).length > 80) throw new Error('invalid team');
    const phase = entryPhase || 'offseason';
    const id = randomUUID();
    const repoRoot = LOLMANAGER_REPO_ROOT;
    const progressFile = path.join('/tmp', `lolmanager-fresh-start-${id}.progress.json`);
    const stdoutFile = path.join('/tmp', `lolmanager-fresh-start-${id}.out.log`);
    const stderrFile = path.join('/tmp', `lolmanager-fresh-start-${id}.err.log`);
    const startedAt = Date.now();

    const packaged = readPackagedLolManagerContext({ startYear: year, team, entryPhase: phase });
    if (packaged) {
        const job = {
            id,
            status: 'done',
            startedAt,
            finishedAt: Date.now(),
            target: { startYear: year, team: String(team), entryPhase: String(phase) },
            progressFile,
            stdoutFile,
            stderrFile,
            progress: { status: 'done', stage: 'package_hit', pct: 100, detail: 'Prebuilt Fresh Start package loaded.' },
            result: packaged
        };
        freshStartJobs.set(id, job);
        return job;
    }

    const current = readLolManagerContext();
    const currentTeam = current.snapshot?.team || {};
    if (
        Number(current.snapshot?.start_year) === year &&
        String(current.snapshot?.entry_phase || 'offseason') === String(phase) &&
        [currentTeam.display_name, currentTeam.canonical_display_name].includes(String(team))
    ) {
        const job = {
            id,
            status: 'done',
            startedAt,
            finishedAt: Date.now(),
            target: { startYear: year, team: String(team), entryPhase: String(phase) },
            progressFile,
            stdoutFile,
            stderrFile,
            progress: { status: 'done', stage: 'already_current', pct: 100, detail: 'Current snapshot already matches requested Fresh Start.' },
            result: current
        };
        freshStartJobs.set(id, job);
        return job;
    }

    readOrExportLolManagerIndex();
    fs.writeFileSync(progressFile, JSON.stringify({
        schema_version: 'fresh_start_generation_progress_v0',
        status: 'running',
        stage: 'spawn_exporter',
        pct: 0,
        detail: `${team} ${year} ${phase} snapshot exporter spawned`,
        updated_at: new Date().toISOString()
    }, null, 2));

    const out = fs.openSync(stdoutFile, 'w');
    const err = fs.openSync(stderrFile, 'w');
    const child = spawn('python3', [
        'data_pipeline_v2/scripts/export_fresh_start_game_snapshot.py',
        '--team', String(team),
        '--start-year', String(year),
        '--entry-phase', String(phase),
        '--export', path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_snapshot.json'),
        '--progress-file', progressFile
    ], { cwd: repoRoot, stdio: ['ignore', out, err] });

    const job = {
        id,
        status: 'running',
        startedAt,
        finishedAt: null,
        target: { startYear: year, team: String(team), entryPhase: String(phase) },
        progressFile,
        stdoutFile,
        stderrFile,
        pid: child.pid,
        result: null,
        error: null
    };
    freshStartJobs.set(id, job);

    child.on('close', (code, signal) => {
        job.finishedAt = Date.now();
        if (code === 0) {
            job.status = 'done';
            job.result = readLolManagerContext();
        } else {
            job.status = 'error';
            job.error = `Fresh Start exporter failed code=${code} signal=${signal || ''}`.trim();
        }
        try { fs.closeSync(out); } catch (_) {}
        try { fs.closeSync(err); } catch (_) {}
    });
    child.on('error', (errObj) => {
        job.finishedAt = Date.now();
        job.status = 'error';
        job.error = errObj.message;
    });
    return job;
};

const freshStartJobStatus = (id) => {
    const job = freshStartJobs.get(id);
    if (!job) return null;
    const progress = readProgressFile(job.progressFile) || job.progress || { status: job.status, stage: job.status, pct: job.status === 'done' ? 100 : 0, detail: '' };
    const elapsedSeconds = ((job.finishedAt || Date.now()) - job.startedAt) / 1000;
    let stderrTail = '';
    if (job.status === 'error') {
        try { stderrTail = fs.readFileSync(job.stderrFile, 'utf8').slice(-4000); } catch (_) {}
    }
    return {
        id: job.id,
        status: job.status,
        target: job.target,
        pid: job.pid,
        elapsedSeconds,
        progress,
        error: job.error,
        stderrTail,
        result: job.status === 'done' ? job.result : undefined
    };
};

setInterval(() => {
    const cutoff = Date.now() - 1000 * 60 * 60;
    for (const [id, job] of freshStartJobs.entries()) {
        if (job.finishedAt && job.finishedAt < cutoff) freshStartJobs.delete(id);
    }
}, 1000 * 60 * 10);

const readRequestJson = (req, maxBytes = 1024 * 64) => new Promise((resolve, reject) => {
    let body = '';
    req.on('data', chunk => {
        body += chunk;
        if (body.length > maxBytes) reject(new Error('request too large'));
    });
    req.on('end', () => {
        try {
            resolve(body ? JSON.parse(body) : {});
        } catch (err) {
            reject(err);
        }
    });
    req.on('error', reject);
});

const runLolManagerSeasonAction = (action, payload) => {
    if (!new Set(['init', 'jump', 'play', 'skip']).has(action)) throw new Error('invalid season action');
    const stdout = execFileSync('python3', ['data_pipeline_v2/scripts/run_season_runtime_action.py', '--action', action], {
        cwd: LOLMANAGER_REPO_ROOT, input: JSON.stringify(payload), encoding: 'utf8',
        maxBuffer: 1024 * 1024 * 16, timeout: 120000
    });
    return JSON.parse(stdout);
};

const generateLolManagerContext = ({ startYear, team, entryPhase }) => {
    const year = Number(startYear);
    if (!Number.isInteger(year) || year < 2010 || year > 2035) throw new Error('invalid startYear');
    if (!team || String(team).length > 80) throw new Error('invalid team');
    const phase = entryPhase || 'offseason';
    const current = readLolManagerContext();
    const currentTeam = current.snapshot?.team || {};
    if (
        Number(current.snapshot?.start_year) === year &&
        String(current.snapshot?.entry_phase || 'offseason') === String(phase) &&
        [currentTeam.display_name, currentTeam.canonical_display_name].includes(String(team))
    ) {
        return current;
    }

    const packaged = readPackagedLolManagerContext({ startYear: year, team, entryPhase: phase });
    if (packaged) return packaged;

    const repoRoot = LOLMANAGER_REPO_ROOT;
    readOrExportLolManagerIndex();
    execFileSync('python3', [
        'data_pipeline_v2/scripts/export_fresh_start_game_snapshot.py',
        '--team', String(team),
        '--start-year', String(year),
        '--entry-phase', String(phase),
        '--export', path.join(LOLMANAGER_WEB_ROOT, 'data', 'fresh_start_snapshot.json')
    ], { cwd: repoRoot, stdio: 'pipe' });
    return readLolManagerContext();
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
    if (req.url === '/lolmanager/api/match-room/catalog' && req.method === 'GET') {
        try {
            sendJson(res, 200, runLolManagerMatchRoom('catalog'));
        } catch (err) {
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    const matchRoomRoute = req.url.match(/^\/lolmanager\/api\/match-room\/(bootstrap|load|command)$/);
    if (matchRoomRoute && req.method === 'POST') {
        readRequestJson(req, 1024 * 1024 * 4)
            .then(payload => sendJson(res, 200, runLolManagerMatchRoom(matchRoomRoute[1], payload)))
            .catch(err => sendJson(res, 400, { error: err.message }));
        return;
    }

    if (req.url === '/lolmanager/api/index' && req.method === 'GET') {
        try {
            sendJson(res, 200, readOrExportLolManagerIndex());
        } catch (err) {
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    if (req.url === '/lolmanager/api/context' && req.method === 'GET') {
        try {
            sendJson(res, 200, readLolManagerContext());
        } catch (err) {
            sendJson(res, 500, { error: err.message });
        }
        return;
    }

    if (req.url === '/lolmanager/api/generate' && req.method === 'POST') {
        readRequestJson(req)
            .then(payload => sendJson(res, 200, generateLolManagerContext(payload)))
            .catch(err => sendJson(res, 400, { error: err.message }));
        return;
    }

    if (req.url === '/lolmanager/api/generate/start' && req.method === 'POST') {
        readRequestJson(req)
            .then(payload => {
                const job = startLolManagerGenerationJob(payload);
                sendJson(res, 202, freshStartJobStatus(job.id));
            })
            .catch(err => sendJson(res, 400, { error: err.message }));
        return;
    }

    if (req.url.startsWith('/lolmanager/api/generate/status') && req.method === 'GET') {
        const requestUrl = new URL(req.url, 'http://localhost');
        const id = requestUrl.searchParams.get('id');
        const status = id ? freshStartJobStatus(id) : null;
        if (!status) sendJson(res, 404, { error: 'job not found' });
        else sendJson(res, 200, status);
        return;
    }

    if (req.url.startsWith('/lolmanager/api/season/') && req.method === 'POST') {
        const action = req.url.slice('/lolmanager/api/season/'.length).split('?')[0];
        readRequestJson(req, 1024 * 1024 * 4)
            .then(payload => sendJson(res, 200, runLolManagerSeasonAction(action, payload)))
            .catch(err => sendJson(res, 400, { error: err.message }));
        return;
    }

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
