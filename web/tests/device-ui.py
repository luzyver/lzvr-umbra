"""Run against local Vite: python web/tests/device-ui.py [--url http://127.0.0.1:5173].

Requires Python Playwright + Chromium. All /api requests are isolated fixtures;
no login, profile, or mutation reaches a real backend. Screenshots stay in .impeccable.
"""
import argparse
import copy
import json
from datetime import datetime, timezone, timedelta
from pathlib import Path
from playwright.sync_api import sync_playwright, expect

args = argparse.ArgumentParser()
args.add_argument('--url', default='http://127.0.0.1:5173')
BASE = args.parse_args().url
OUT = Path(__file__).resolve().parents[2] / '.impeccable' / 'review'
OUT.mkdir(parents=True, exist_ok=True)
now = datetime.now(timezone.utc)
stamp = (now - timedelta(seconds=62)).isoformat()
CLIENTS = [dict(id=i, name=name, address=f'10.8.0.{i+1}/32', enabled=i != 4,
    has_private_key=i != 5, public_key='demo-public-key', allowed_ips=['0.0.0.0/0'],
    dns=['10.8.0.1'], mtu=1420, keepalive=25, adblock=True, created_at=stamp, updated_at=stamp)
    for i, name in enumerate(['Laptop kantor', 'iPhone pribadi', 'Server rumah', 'Tablet keluarga', 'Router ruang kerja', 'Laptop cadangan'], 1)]
STATUS = dict(wg_backend='fake', initialized=True, interface='wg0', online=3, total=len(CLIENTS),
    rx_bytes=18400000000, tx_bytes=4200000000, time=stamp,
    clients=[dict(id=c['id'], name=c['name'], address=c['address'], enabled=c['enabled'],
        online=c['id'] in [1, 2, 3], rx_bytes=c['id']*1234000000, tx_bytes=c['id']*280000000,
        last_handshake=(now-timedelta(seconds=c['id']*20 if c['id'] < 4 else 7200)).isoformat(),
        endpoint='203.0.113.12:51820') for c in CLIENTS])

def setup(page, empty=False, fail=False, long_name=False, feed_error=False):
    clients = [] if empty else copy.deepcopy(CLIENTS)
    status = copy.deepcopy(STATUS)
    if empty:
        status.update(clients=[], total=0, online=0, rx_bytes=0, tx_bytes=0)
    if long_name:
        clients[0]['name'] = 'Laptop operasional cabang dengan nama sangat panjang ' * 4
        status['clients'][0]['name'] = clients[0]['name']
        clients[0]['address'] = 'fd00:1234:5678:abcd:1234:5678:abcd:1234/128'
        status['clients'][0]['address'] = clients[0]['address']
    requests = []
    def api(route):
        path = route.request.url.split('/api', 1)[1].split('?')[0]
        method = route.request.method
        requests.append((method, path))
        if (fail or feed_error) and path in ['/status', '/clients']:
            route.fulfill(status=503, json={'error': 'Layanan sedang tidak tersedia.'})
            return
        data = {'/setup/status': dict(needs_admin=False, initialized=True),
            '/auth/me': dict(id=1, username='admin', adguard=True, fake_wg=True),
            '/status': status, '/clients': clients,
            '/defaults': dict(dns=['10.8.0.1'], allowed_ips=['0.0.0.0/0'], keepalive=25, mtu=1420),
            '/server': dict(interface='wg0', address='10.8.0.1/24', endpoint_host='vpn.example.net',
                endpoint_port=51820, listen_port=51820, mtu=1420, out_interface='eth0', public_key='demo', initialized=True),
            '/adguard/status': dict(configured=True, reachable=True, status=dict(running=True, protection_enabled=True, version='v0.107', dns_addresses=['10.8.0.1'], dns_port=53))}
        if path.startswith('/clients/'):
            cid = int(path.split('/')[2])
            client = next((c for c in clients if c['id'] == cid), {})
            data[path] = client
            if method == 'PUT':
                client.update(route.request.post_data_json)
                data[path] = dict(client=client, warnings=[], profile_changed=False)
            elif method == 'DELETE':
                clients.remove(client)
                data[path] = dict(warnings=[])
            if path.endswith('/qr.png'):
                route.fulfill(status=503, body='Intentional QR failure fixture')
                return
        if path == '/clients' and method == 'POST':
            client = dict(CLIENTS[0], id=99, **route.request.post_data_json)
            clients.append(client)
            data[path] = dict(client=client, warnings=[], profile_changed=False)
        route.fulfill(json=data.get(path, {}))
    page.route('**/api/**', api)
    page.add_init_script('''window.__feeds=[]; window.EventSource=class {
      constructor(){this.listeners={};window.__feeds.push(this);let n=0;
        this.timer=setInterval(()=>{
          if(window.__pauseFeed)return;
          const s=window.__status; const old=Date.parse(s.time);
          const t=n<30 ? old+2000 : Math.max(old+1,Date.now());n++;
          const delta=(t-old)/1000;s.time=new Date(t).toISOString();
          s.clients.forEach(c=>{if(c.online && c.enabled && !window.__idleFeed){
            const rx=Math.round((200000+Math.sin(n*.6+c.id)*100000)*delta);
            const tx=Math.round((60000+Math.sin(n*.3)*20000)*delta);
            c.rx_bytes+=rx;c.tx_bytes+=tx;s.rx_bytes+=rx;s.tx_bytes+=tx;
          }});
          // A failed client read reaches the page as an event with an error and no clients.
          const out=window.__feedError ? {...s,clients:[],online:0,total:0,error:'baca klien gagal'} : s;
          this.listeners.status?.({data:JSON.stringify(out)});
        },50)}
      addEventListener(n,fn){this.listeners[n]=fn}close(){clearInterval(this.timer);this.closed=true}
    }; window.__status=''' + json.dumps(status) + ';window.__pauseFeed=' + json.dumps(fail) + ';window.__feedError=' + json.dumps(feed_error))
    return requests

def no_overflow(page):
    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), 'Page overflows horizontally'

def capture(page, name, full=True):
    page.evaluate('window.scrollTo(0,0)')
    page.wait_for_timeout(280)
    no_overflow(page)
    page.screenshot(path=str(OUT / name), full_page=full)

with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page(viewport=dict(width=1440, height=1000), device_scale_factor=1)
    page.clock.install()
    errors = []
    page.on('pageerror', lambda error: errors.append(str(error)))
    requests = setup(page)
    page.goto(BASE)
    page.get_by_role('heading', name='Ringkasan', exact=True).wait_for()
    page.wait_for_timeout(1800)
    capture(page, 'desktop.png')
    # Online with traffic keeps the online class and adds busy; without traffic it is plain online.
    expect(page.locator('.device-status.online.busy')).to_have_count(3)
    expect(page.locator('.device-status.online').first).to_contain_text('Online')
    expect(page.locator('.port.online.busy')).to_have_count(3)
    page.evaluate('window.__idleFeed=true')
    expect(page.locator('.device-status.busy')).to_have_count(0)
    expect(page.locator('.port.busy')).to_have_count(0)
    expect(page.locator('.device-status.online')).to_have_count(3)
    expect(page.locator('.port.online')).to_have_count(3)
    page.evaluate('window.__idleFeed=false')
    expect(page.locator('.device-status.online.busy')).to_have_count(3)
    page.locator('.device-filters').get_by_role('button', name='Offline').click()
    expect(page.locator('.device-table tbody tr')).to_have_count(2)
    page.locator('.device-filters').get_by_role('button', name='Nonaktif').click()
    expect(page.locator('.device-table tbody tr')).to_have_count(1)
    page.locator('.device-filters').get_by_role('button', name='Semua').click()
    page.get_by_role('textbox', name='Cari perangkat').fill('  LAPTOP  ')
    expect(page.locator('.device-table tbody tr')).to_have_count(2)
    page.get_by_role('textbox', name='Cari perangkat').fill('no-match')
    expect(page.get_by_text('Tidak ada perangkat yang sesuai.')).to_be_visible()
    page.get_by_role('button', name='Hapus filter').click()
    page.get_by_role('button', name='Detail Laptop kantor, 10.8.0.2/32', exact=True).click()
    expect(page.get_by_role('dialog')).to_be_visible()
    expect(page.get_by_role('link', name='Aktivitas jaringan', exact=True)).to_be_visible()
    capture(page, 'drawer.png', False)
    page.get_by_role('button', name='Profil & QR').click()
    expect(page.get_by_text('QR gagal dimuat.')).to_be_visible()
    page.wait_for_timeout(700)
    expect(page.get_by_role('button', name='Profil & QR')).to_have_attribute('aria-pressed', 'true')
    assert requests.count(('GET', '/clients/1')) == 1, 'Live updates refetch selected profile'
    expect(page.get_by_role('link', name='Unduh file .conf')).to_have_attribute('href', '/api/clients/1/config')
    page.keyboard.press('Escape')
    expect(page.get_by_role('dialog')).not_to_be_visible()
    expect(page.get_by_role('button', name='Detail Laptop kantor, 10.8.0.2/32', exact=True)).to_be_focused()
    # A silent connection must also stop claiming that old statuses are current.
    page.evaluate('window.__pauseFeed=true')
    page.clock.fast_forward(16000)
    expect(page.locator('.device-status').filter(has_text='Belum diketahui')).to_have_count(5)
    page.evaluate('window.__pauseFeed=false')
    page.clock.fast_forward(100)
    expect(page.locator('.device-status.online')).to_have_count(3)
    page.evaluate("window.__pauseFeed=true;window.__feeds.forEach(f=>f.onerror?.())")
    expect(page.locator('.device-status').filter(has_text='Belum diketahui')).to_have_count(5)
    page.locator('.device-filters').get_by_role('button', name='Offline').click()
    expect(page.get_by_text('Tidak ada perangkat yang sesuai.')).to_be_visible()
    page.get_by_role('button', name='Hapus filter').click()
    page.evaluate('window.__pauseFeed=false')
    expect(page.locator('.device-status.online')).to_have_count(3)
    page.evaluate("window.dispatchEvent(new CustomEvent('umbra:set-theme',{detail:'dark'}))")
    capture(page, 'desktop-dark.png')
    page.evaluate("window.dispatchEvent(new CustomEvent('umbra:set-theme',{detail:'light'}))")
    for width in [1280, 1024, 768, 390, 320]:
        page.set_viewport_size(dict(width=width, height=844))
        no_overflow(page)
    page.set_viewport_size(dict(width=390, height=844))
    capture(page, 'mobile.png')
    accessible_table = page.locator('.device-table').aria_snapshot()
    assert 'columnheader "Perangkat"' in accessible_table
    assert 'columnheader "Masuk"' in accessible_table and 'columnheader "Keluar"' in accessible_table
    assert 'Online, ada trafik' in accessible_table
    assert 'Detail Laptop kantor, 10.8.0.2/32' in accessible_table
    page.get_by_role('button', name='Detail Laptop kantor, 10.8.0.2/32', exact=True).click()
    capture(page, 'mobile-drawer.png', False)
    assert page.get_by_role('dialog').bounding_box()['width'] == 390
    page.keyboard.press('Escape')
    # Client management: native dialog transitions, mutations and deep links.
    page.set_viewport_size(dict(width=1440, height=1000))
    page.goto(BASE + '/clients?device=1')
    expect(page.get_by_role('dialog')).to_be_visible()
    page.get_by_role('switch', name='Akses VPN').click()
    expect(page.get_by_role('switch', name='Akses VPN')).to_have_attribute('aria-checked', 'false')
    page.get_by_role('button', name='Ubah perangkat', exact=True).click()
    page.get_by_role('textbox', name='Nama perangkat').fill('Laptop diperbarui')
    page.get_by_role('button', name='Simpan', exact=True).click()
    expect(page.get_by_role('button', name='Detail Laptop diperbarui')).to_be_visible()
    page.get_by_role('button', name='Tambah perangkat', exact=True).click()
    expect(page.get_by_role('dialog')).to_be_visible()
    page.get_by_role('textbox', name='Nama perangkat').fill('Perangkat pengujian')
    page.get_by_role('button', name='Simpan', exact=True).click()
    expect(page.get_by_role('dialog', name='Profil Perangkat pengujian')).to_be_visible()
    page.keyboard.press('Escape')
    page.get_by_role('button', name='Detail Perangkat pengujian, 10.8.0.2/32', exact=True).click()
    page.get_by_role('button', name='Hapus', exact=True).click()
    page.get_by_role('dialog', name='Hapus perangkat').get_by_role('button', name='Hapus', exact=True).click()
    expect(page.get_by_role('button', name='Detail Perangkat pengujian, 10.8.0.2/32', exact=True)).to_have_count(0)
    capture(page, 'clients.png')
    page.set_viewport_size(dict(width=390, height=844))
    capture(page, 'clients-mobile.png')
    page.get_by_role('button', name='Detail Router ruang kerja, 10.8.0.6/32', exact=True).click()
    page.get_by_role('button', name='Profil & QR').click()
    expect(page.get_by_role('button', name='Buat ulang kunci')).to_be_visible()
    page.keyboard.press('Escape')
    page.get_by_role('button', name='Impor', exact=True).click()
    expect(page.get_by_role('dialog')).to_be_visible()
    page.keyboard.press('Escape')
    # An error event before the first valid snapshot must not be read as an empty subnet.
    early = browser.new_page(viewport=dict(width=1280, height=900))
    early.on('pageerror', lambda error: errors.append(str(error)))
    setup(early, feed_error=True)
    early.goto(BASE)
    caption = early.locator('.ports-caption')
    expect(caption).to_contain_text('Pemakaian alamat belum diketahui')
    early.wait_for_timeout(400)
    expect(caption).not_to_contain_text('terpakai')
    early.evaluate('window.__feedError=false')
    expect(caption).to_contain_text('6 terpakai')
    early.evaluate('window.__feedError=true')
    expect(early.locator('.inline-notice')).to_be_visible()
    expect(caption).to_contain_text('6 terpakai')
    early.close()
    for mode in ['empty', 'fail', 'long_name']:
        other = browser.new_page(viewport=dict(width=390, height=844))
        other.on('pageerror', lambda error: errors.append(str(error)))
        setup(other, **{mode: True})
        other.goto(BASE if mode != 'fail' else BASE + '/clients')
        other.get_by_role('heading', level=1).wait_for()
        other.wait_for_timeout(400)
        no_overflow(other)
        if mode == 'empty':
            expect(other.get_by_role('link', name='Tambah perangkat pertama')).to_be_visible()
            other.get_by_role('link', name='Tambah perangkat pertama').click()
            expect(other.get_by_role('dialog', name='Tambah perangkat', exact=True)).to_be_visible()
            other.keyboard.press('Escape')
            other.goto(BASE + '/clients')
            other.get_by_text('Tambah perangkat pertama', exact=False).click()
            expect(other.get_by_role('dialog', name='Tambah perangkat', exact=True)).to_be_visible()
            expect(other.get_by_role('textbox', name='Nama perangkat')).to_be_visible()
            other.keyboard.press('Escape')
        elif mode == 'fail':
            expect(other.get_by_text('Daftar perangkat belum dapat dimuat.')).to_be_visible()
            expect(other.get_by_role('link', name='Tambah perangkat pertama')).to_have_count(0)
        else:
            other.get_by_role('button', name='Detail Laptop operasional', exact=False).click()
            no_overflow(other)
        other.close()
    assert not errors, errors
    browser.close()
    print(json.dumps(dict(result='passed', scenarios=['desktop/mobile/light/dark', 'search/filters', 'drawer/keyboard/focus', 'live/stale/recovery', 'profile stability', 'edit/create/delete isolated fixtures', 'import/keyless', 'empty/error/long IPv6'], page_errors=errors)))
