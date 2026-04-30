import { execSync } from 'child_process';
import http from 'http';

/**
 * MistVault Dashboard Server
 * 리소스 점유를 최소화하기 위해 별도의 프레임워크 없이 Node.js 네이티브로 구동
 */

const PORT = 8088; // 대시보드 포트

const getListeningPorts = () => {
    try {
        // lsof를 이용해 LISTEN 상태인 포트와 프로세스 명 확인
        const output = execSync("lsof -i -P -n | grep LISTEN").toString();
        const lines = output.trim().split('\n');
        const ports = lines.map(line => {
            const parts = line.split(/\s+/);
            const process = parts[0];
            const address = parts[8];
            const port = address.split(':').pop();
            return { process, port };
        });

        // 중복 제거 및 정렬
        return Array.from(new Map(ports.map(p => [p.port, p])).values())
                    .sort((a, b) => parseInt(a.port) - parseInt(b.port));
    } catch (e) {
        return [];
    }
};

const getTailscaleIP = () => {
    try {
        // ifconfig에서 100.으로 시작하는 inet 주소를 직접 추출
        const ip = execSync("ifconfig | grep 'inet 100.' | awk '{print $2}'").toString().trim();
        if (ip && ip.startsWith('100.')) {
            return ip;
        }
        
        // 위 방법이 실패하면 기존 os 모듈 방식 (백업)
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

const server = http.createServer((req, res) => {
    const ports = getListeningPorts();
    const tailscaleIP = getTailscaleIP();
    
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' });
    
    let html = `
    <!DOCTYPE html>
    <html>
    <head>
        <title>👻 MistVault Dashboard</title>
        <meta name="viewport" content="width=device-width, initial-scale=1">
        <style>
            body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #121212; color: #e0e0e0; padding: 20px; }
            h1 { color: #bb86fc; margin-bottom: 5px; }
            .ip-info { color: #666; font-family: monospace; margin-bottom: 20px; font-size: 0.9em; }
            .card { background: #1e1e1e; border-radius: 8px; padding: 15px; margin-bottom: 10px; border: 1px solid #333; display: flex; justify-content: space-between; align-items: center; }
            .process { font-weight: bold; color: #03dac6; }
            .port { font-family: monospace; background: #333; padding: 2px 6px; border-radius: 4px; }
            a { color: #bb86fc; text-decoration: none; }
            a:hover { text-decoration: underline; }
            .refresh { margin-bottom: 20px; display: inline-block; padding: 8px 16px; background: #bb86fc; color: #000; border-radius: 4px; font-weight: bold; }
        </style>
    </head>
    <body>
        <h1>👻 MistVault Dashboard</h1>
        <div class="ip-info">Tailscale IP: ${tailscaleIP}</div>
        <a href="javascript:location.reload()" class="refresh">새로고침</a>
        <div>
    `;

    ports.forEach(p => {
        // 테일스케일 IP 기반의 실제 외부 접속 주소 생성
        const url = `http://${tailscaleIP}:${p.port}`;
        html += `
            <div class="card">
                <div>
                    <span class="process">${p.process}</span>
                    <span class="port">:${p.port}</span>
                </div>
                <a href="${url}" target="_blank">접속하기 ➔</a>
            </div>
        `;
    });

    html += `
        </div>
        <p style="font-size: 0.8em; color: #666; margin-top: 20px;">Tailscale 전용 도메인으로 접속 시 포트 번호만 붙여서 사용해.</p>
    </body>
    </html>
    `;
    
    res.end(html);
});

server.listen(PORT, '0.0.0.0', () => {
    console.log(`MistVault Dashboard running at http://0.0.0.0:${PORT}`);
});
