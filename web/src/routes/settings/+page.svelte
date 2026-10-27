<script lang="ts">
	import { get, put, post, errMsg, type Defaults, type ServerConfig } from '#lib/api.ts';
	import { splitList } from '#lib/format.ts';
	import { toast, warnAll } from '#lib/toast.svelte.ts';
	import ServerForm from '#lib/ServerForm.svelte';
	import FirewallPanel from '#lib/FirewallPanel.svelte';

	let server = $state<ServerConfig | null>(null);
	let error = $state('');

	// Bawaan klien baru
	let d = $state({ dns: '', allowed_ips: '', keepalive: '', mtu: '' });
	let dBusy = $state(false);
	let dError = $state('');

	// Ganti kata sandi
	let pw = $state({ current: '', next: '', repeat: '' });
	let pwBusy = $state(false);
	let pwError = $state('');

	function fillDefaults(x: Defaults) {
		d = {
			dns: x.dns.join(', '),
			allowed_ips: x.allowed_ips.join(', '),
			keepalive: String(x.keepalive ?? ''),
			mtu: x.mtu ? String(x.mtu) : ''
		};
	}

	async function load() {
		try {
			const [s, x] = await Promise.all([get<ServerConfig>('/server'), get<Defaults>('/defaults')]);
			server = s;
			fillDefaults(x);
		} catch (e) {
			error = errMsg(e);
		}
	}
	load();

	function serverSaved(s: ServerConfig, profileChanged: boolean) {
		server = s;
		if (profileChanged) toast('Profil semua perangkat perlu diunduh ulang.', 'warn', 8000);
	}

	async function saveDefaults(e: SubmitEvent) {
		e.preventDefault();
		dBusy = true;
		dError = '';
		try {
			const r = await put<Defaults & { warnings?: string[] }>('/defaults', {
				dns: splitList(d.dns),
				allowed_ips: splitList(d.allowed_ips),
				keepalive: Number(d.keepalive) || 0,
				mtu: Number(d.mtu) || 0
			});
			warnAll(r?.warnings);
			if (r?.dns) fillDefaults(r);
			toast('Bawaan klien disimpan. Klien yang memakai bawaan perlu mengunduh ulang profil.');
		} catch (e) {
			dError = errMsg(e);
		} finally {
			dBusy = false;
		}
	}

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		pwError = '';
		if (pw.next !== pw.repeat) {
			pwError = 'Kata sandi baru dan ulangannya tidak sama.';
			return;
		}
		pwBusy = true;
		try {
			await post('/auth/password', { current: pw.current, new: pw.next });
			pw = { current: '', next: '', repeat: '' };
			toast('Kata sandi diganti.');
		} catch (e) {
			pwError = errMsg(e);
		} finally {
			pwBusy = false;
		}
	}
</script>

<svelte:head><title>Pengaturan · Umbra</title></svelte:head>

<h1 class="mb-4 text-xl font-semibold">Pengaturan</h1>

{#if error}
	<div class="card text-sm text-red-600">{error}</div>
{:else if !server}
	<p class="text-sm text-zinc-500">Memuat…</p>
{:else}
	<div class="space-y-6">
		<section class="card">
			<h2 class="font-semibold">Server WireGuard</h2>
			<p class="hint mb-4 mt-1">
				Perubahan langsung diterapkan. Mengubah alamat, port atau endpoint membuat profil di perangkat
				perlu diunduh ulang.
			</p>
			{#if server.public_key}
				<p class="mb-4 text-xs text-zinc-500">
					Kunci publik server: <span class="break-all font-mono">{server.public_key}</span>
				</p>
			{/if}
			<ServerForm {server} onsaved={serverSaved} />
		</section>

		<section class="card">
			<h2 class="font-semibold">Bawaan klien baru</h2>
			<p class="hint mb-4 mt-1">Dipakai bila kolom di klien dibiarkan kosong.</p>
			<form class="grid gap-4 sm:grid-cols-2" onsubmit={saveDefaults}>
				<div>
					<label class="label" for="d-dns">DNS</label>
					<input id="d-dns" class="input font-mono" bind:value={d.dns} placeholder="10.8.0.1" />
					<p class="hint">Isi IP server WireGuard bila memakai AdGuard.</p>
				</div>
				<div>
					<label class="label" for="d-aips">Allowed IPs</label>
					<input id="d-aips" class="input font-mono" bind:value={d.allowed_ips} placeholder="0.0.0.0/0, ::/0" />
					<p class="hint">0.0.0.0/0 = semua internet lewat VPN.</p>
				</div>
				<div>
					<label class="label" for="d-ka">Keepalive (detik)</label>
					<input id="d-ka" class="input" type="number" min="0" max="3600" bind:value={d.keepalive} />
					<p class="hint">25 disarankan supaya HP tetap tersambung di balik NAT. 0 = mati.</p>
				</div>
				<div>
					<label class="label" for="d-mtu">MTU</label>
					<input id="d-mtu" class="input" type="number" min="0" max="1500" bind:value={d.mtu} placeholder="otomatis" />
				</div>
				{#if dError}<p class="text-sm text-red-600 sm:col-span-2">{dError}</p>{/if}
				<div class="sm:col-span-2">
					<button class="btn-primary" disabled={dBusy}>{dBusy ? 'Menyimpan…' : 'Simpan bawaan'}</button>
				</div>
			</form>
		</section>

		<section class="card">
			<h2 class="mb-4 font-semibold">Firewall & NAT</h2>
			<FirewallPanel />
		</section>

		<section class="card">
			<h2 class="mb-4 font-semibold">Ganti kata sandi</h2>
			<form class="grid max-w-md gap-4" onsubmit={changePassword}>
				<input type="text" autocomplete="username" class="hidden" aria-hidden="true" tabindex="-1" />
				<div>
					<label class="label" for="pw-cur">Kata sandi sekarang</label>
					<input id="pw-cur" class="input" type="password" bind:value={pw.current} autocomplete="current-password" required />
				</div>
				<div>
					<label class="label" for="pw-new">Kata sandi baru</label>
					<input id="pw-new" class="input" type="password" minlength="10" bind:value={pw.next} autocomplete="new-password" required />
					<p class="hint">Minimal 10 karakter.</p>
				</div>
				<div>
					<label class="label" for="pw-rep">Ulangi kata sandi baru</label>
					<input id="pw-rep" class="input" type="password" bind:value={pw.repeat} autocomplete="new-password" required />
				</div>
				{#if pwError}<p class="text-sm text-red-600">{pwError}</p>{/if}
				<div>
					<button class="btn-primary" disabled={pwBusy}>{pwBusy ? 'Menyimpan…' : 'Ganti kata sandi'}</button>
				</div>
			</form>
		</section>
	</div>
{/if}
