"""Buat ulang tangkapan layar README: python web/tests/screenshots.py [--url http://127.0.0.1:5173].

Butuh frontend berjalan (cd web && npm run dev), Python, Playwright, dan Chromium. Semua permintaan API
diganti data contoh, jadi backend tidak perlu dijalankan. Hasil disimpan di docs/screenshots/.
"""
import argparse
import json
from datetime import datetime, timezone, timedelta
from pathlib import Path
from playwright.sync_api import sync_playwright, expect

args = argparse.ArgumentParser()
args.add_argument('--url', default='http://127.0.0.1:5173')
BASE = args.parse_args().url.rstrip('/')
OUT = Path(__file__).resolve().parents[2] / 'docs' / 'screenshots'
OUT.mkdir(parents=True, exist_ok=True)

now = datetime.now(timezone.utc)
stamp = now.isoformat()
names = ['Laptop kantor', 'iPhone pribadi', 'Server rumah', 'Tablet keluarga', 'Router ruang kerja', 'Laptop cadangan']
clients = [dict(id=i, name=name, address=f'10.8.0.{i+1}/32', enabled=i != 4, has_private_key=True,
    public_key='contoh-kunci-publik', allowed_ips=['0.0.0.0/0'], dns=['10.8.0.1'], mtu=1420, keepalive=25,
    adblock=True, created_at=stamp, updated_at=stamp) for i, name in enumerate(names, 1)]
status = dict(wg_backend='kernel', initialized=True, interface='wg0', online=3, total=len(clients),
    rx_bytes=18400000000, tx_bytes=4200000000, time=stamp,
    clients=[dict(id=c['id'], name=c['name'], address=c['address'], enabled=c['enabled'],
        online=c['id'] in [1, 2, 3], rx_bytes=c['id'] * 1234000000, tx_bytes=c['id'] * 280000000,
        last_handshake=(now - timedelta(seconds=c['id'] * 20 if c['id'] < 4 else 7200)).isoformat(),
        endpoint='203.0.113.12:51820') for c in clients])
data = {
    '/setup/status': dict(needs_admin=False, initialized=True),
    '/status': status, '/clients': clients,
    '/defaults': dict(dns=['10.8.0.1'], allowed_ips=['0.0.0.0/0'], keepalive=25, mtu=1420),
    '/server': dict(interface='wg0', address='10.8.0.1/24', endpoint_host='vpn.example.net', endpoint_port=51820,
        listen_port=51820, mtu=1420, out_interface='eth0', public_key='contoh', initialized=True),
    '/adguard/status': dict(configured=True, reachable=True, status=dict(running=True, protection_enabled=True,
        version='v0.107', dns_addresses=['10.8.0.1'], dns_port=53)),
    '/activity': dict(items=[dict(id=i, client_id=1 + i % 3, client_name=names[i % 3], kind=['connect', 'disconnect', 'roam'][i % 3],
        endpoint='203.0.113.12:51820', duration=3600 * i, rx_bytes=10**8 * i, tx_bytes=10**7 * i,
        at=(now - timedelta(minutes=17 * i)).isoformat()) for i in range(1, 9)], more=False),
}
# Umpan status tiruan: 40 titik pertama dimajukan 2 detik per titik supaya grafik langsung terisi,
# waktunya selalu maju dari cuplikan /status awal agar laju tidak melonjak.
feed = '''window.EventSource = class {
  constructor() { this.listeners = {}; let n = 0; this.timer = setInterval(() => {
    const s = window.__status; const old = Date.parse(s.time);
    const t = n < 40 ? old + 2000 : Math.max(old + 1, Date.now()); n++;
    const delta = (t - old) / 1000; s.time = new Date(t).toISOString();
    s.clients.forEach(c => { if (c.online && c.enabled) {
      const rx = Math.round((420000 + Math.sin(n * .35 + c.id) * 260000) * delta);
      const tx = Math.round((110000 + Math.sin(n * .2 + c.id) * 50000) * delta);
      c.rx_bytes += rx; c.tx_bytes += tx; s.rx_bytes += rx; s.tx_bytes += tx;
    } });
    this.listeners.status?.({ data: JSON.stringify(s) });
  }, 50); }
  addEventListener(n, fn) { this.listeners[n] = fn; } close() { clearInterval(this.timer); }
}; window.__status = ''' + json.dumps(status)

def open_page(browser, width, height, theme, signed_in=True):
    page = browser.new_page(viewport=dict(width=width, height=height), device_scale_factor=2)
    def api(route):
        path = route.request.url.split('/api', 1)[1].split('?')[0]
        if path == '/auth/me':
            if signed_in:
                route.fulfill(json=dict(id=1, username='admin', adguard=True, fake_wg=False))
            else:
                route.fulfill(status=401, json=dict(error='Belum masuk'))
        elif path.startswith('/clients/') and path.split('/')[2].isdigit() and path.count('/') == 2:
            route.fulfill(json=next(c for c in clients if c['id'] == int(path.split('/')[2])))
        elif path in data:
            route.fulfill(json=data[path])
        else:
            route.fulfill(status=404, json=dict(error='Tidak ada di data contoh'))
    page.route('**/api/**', api)
    page.add_init_script(feed)
    page.add_init_script(f"try {{ localStorage.setItem('umbra-theme', '{theme}') }} catch {{}}")
    return page

def shot(page, name, theme):
    page.evaluate(f"document.documentElement.dataset.theme = '{theme}'")
    page.wait_for_timeout(3000)
    page.screenshot(path=str(OUT / name))
    print(name)

detail = dict(name='Detail Laptop kantor, 10.8.0.2/32', exact=True)

with sync_playwright() as p:
    browser = p.chromium.launch()
    for theme in ['light', 'dark']:
        page = open_page(browser, 1280, 860, theme)
        page.goto(BASE + '/')
        page.get_by_role('heading', name='Ringkasan', exact=True).wait_for()
        shot(page, f'overview-{theme}.png', theme)
        if theme == 'light':
            page.get_by_role('button', **detail).click()
            expect(page.get_by_role('dialog')).to_be_visible()
            shot(page, 'device-detail.png', theme)
            page.keyboard.press('Escape')
            page.goto(BASE + '/activity')
            page.get_by_role('button', name='Sesi VPN').click()
            shot(page, 'activity.png', theme)
        page.close()
    page = open_page(browser, 390, 780, 'dark')
    page.goto(BASE + '/')
    page.get_by_role('heading', name='Ringkasan', exact=True).wait_for()
    shot(page, 'mobile-overview.png', 'dark')
    page.close()
    page = open_page(browser, 390, 780, 'light')
    page.goto(BASE + '/')
    page.get_by_role('button', **detail).click()
    expect(page.get_by_role('dialog')).to_be_visible()
    shot(page, 'mobile-device.png', 'light')
    page.close()
    page = open_page(browser, 1280, 860, 'light', signed_in=False)
    page.goto(BASE + '/login')
    page.get_by_role('heading', name='Masuk').wait_for()
    shot(page, 'login.png', 'light')
    browser.close()
