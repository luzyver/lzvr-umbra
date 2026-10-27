<script lang="ts">
	import { untrack } from 'svelte';
	import { put, errMsg, type ServerConfig, type SystemCheck } from './api';
	import { toast, warnAll } from './toast.svelte';

	let {
		server,
		check = null,
		submitLabel = 'Simpan',
		onsaved
	}: {
		server: ServerConfig;
		check?: SystemCheck | null;
		submitLabel?: string;
		onsaved: (s: ServerConfig, profileChanged: boolean) => void;
	} = $props();

	// Salinan lokal supaya form bisa diubah tanpa menyentuh data asal.
	let f = $state(untrack(() => ({ ...server })));
	let busy = $state(false);
	let error = $state('');

	$effect.pre(() => {
		// Susun di variabel lokal: membaca `f` di sini membuat efek memicu dirinya sendiri tanpa henti.
		const next = { ...server };
		// Isi otomatis dari hasil cek sistem saat setup pertama.
		if (!server.initialized && check) {
			if (!next.endpoint_host && check.public_ip) next.endpoint_host = check.public_ip;
			if (!next.out_interface && check.default_iface) next.out_interface = check.default_iface;
		}
		f = next;
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const r = await put<{ server: ServerConfig; profile_changed: boolean; warnings: string[] }>(
				'/server',
				{
					interface: f.interface.trim(),
					address: f.address.trim(),
					listen_port: Number(f.listen_port),
					endpoint_host: f.endpoint_host.trim(),
					endpoint_port: Number(f.endpoint_port) || 0,
					mtu: Number(f.mtu) || 0,
					out_interface: f.out_interface.trim()
				}
			);
			warnAll(r.warnings);
			if (server.initialized) toast('Pengaturan server disimpan dan langsung diterapkan.');
			onsaved(r.server, r.profile_changed);
		} catch (e) {
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}
</script>

<form class="grid gap-4 sm:grid-cols-2" onsubmit={submit}>
	<div>
		<label class="label" for="sf-host">Alamat publik (endpoint)</label>
		<input id="sf-host" class="input" bind:value={f.endpoint_host} placeholder="vpn.contoh.id" required />
		<p class="hint">Domain atau IP yang dipakai HP/laptop untuk tersambung. Jika memakai domain di Cloudflare, matikan proxy (awan abu-abu), karena WireGuard memakai UDP.</p>
	</div>
	<div>
		<label class="label" for="sf-port">Port UDP</label>
		<input id="sf-port" class="input" type="number" min="1" max="65535" bind:value={f.listen_port} required />
		<p class="hint">Buka port ini (UDP) di firewall penyedia VPS / router.</p>
	</div>
	<div>
		<label class="label" for="sf-addr">Alamat server di VPN</label>
		<input id="sf-addr" class="input font-mono" bind:value={f.address} placeholder="10.8.0.1/24" required />
		<p class="hint">Klien akan mendapat IP dari subnet ini (10.8.0.2, 10.8.0.3, …).</p>
	</div>
	<div>
		<label class="label" for="sf-out">Interface internet</label>
		<input id="sf-out" class="input font-mono" bind:value={f.out_interface} placeholder="eth0" required />
		{#if check?.default_iface}
			<p class="hint">Terdeteksi: <span class="font-mono">{check.default_iface}</span></p>
		{/if}
	</div>
	<details class="sm:col-span-2">
		<summary class="cursor-pointer text-sm text-zinc-500">Pengaturan lanjutan</summary>
		<div class="mt-3 grid gap-4 sm:grid-cols-3">
			<div>
				<label class="label" for="sf-if">Nama interface</label>
				<input id="sf-if" class="input font-mono" bind:value={f.interface} disabled={server.initialized} required />
				{#if server.initialized}<p class="hint">Tidak bisa diganti setelah setup.</p>{/if}
			</div>
			<div>
				<label class="label" for="sf-eport">Port publik</label>
				<input id="sf-eport" class="input" type="number" min="0" max="65535" bind:value={f.endpoint_port} />
				<p class="hint">0 = sama dengan port UDP. Isi bila ada port forwarding berbeda.</p>
			</div>
			<div>
				<label class="label" for="sf-mtu">MTU server</label>
				<input id="sf-mtu" class="input" type="number" min="0" max="1500" bind:value={f.mtu} />
				<p class="hint">0 = otomatis.</p>
			</div>
		</div>
	</details>
	{#if error}
		<p class="text-sm text-red-600 sm:col-span-2">{error}</p>
	{/if}
	<div class="sm:col-span-2">
		<button class="btn-primary" disabled={busy}>{busy ? 'Menerapkan…' : submitLabel}</button>
	</div>
</form>
