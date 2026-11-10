<script lang="ts">
	// QR + tombol unduh profil klien. QR diambil sebagai blob karena endpoint butuh sesi.
	import type { Client } from './api';

	let { client, showActivity = true }: { client: Client; showActivity?: boolean } = $props();

	let qr = $state('');
	let failed = $state(false);

	$effect(() => {
		const id = client.id;
		const stamp = client.updated_at;
		let url = '';
		let active = true;
		const controller = new AbortController();
		qr = '';
		failed = false;
		fetch(`/api/clients/${id}/qr.png?v=${encodeURIComponent(stamp)}`, { credentials: 'same-origin', signal: controller.signal })
			.then((r) => (r.ok ? r.blob() : Promise.reject()))
			.then((b) => {
				if (!active) return;
				url = URL.createObjectURL(b);
				qr = url;
			})
			.catch(() => { if (active) failed = true; });
		return () => {
			active = false;
			controller.abort();
			if (url) URL.revokeObjectURL(url);
		};
	});
</script>

<div class="flex flex-col items-center gap-3">
	{#if qr}
		<img src={qr} alt="QR profil {client.name}" class="w-64 rounded bg-white p-2" />
	{:else if failed}
		<p class="text-sm text-[var(--red)]">QR gagal dimuat.</p>
	{:else}
		<div class="h-64 w-64 animate-pulse rounded bg-[var(--panel-alt)]"></div>
	{/if}
	<p class="hint text-center">Pindai dengan aplikasi WireGuard di HP (Tambah tunnel → Pindai QR).</p>
	<a class="btn-ghost" href="/api/clients/{client.id}/config" download>Unduh file .conf</a>
	{#if showActivity}<a class="btn-ghost" href="/activity?client={client.id}">Aktivitas jaringan</a>{/if}
</div>
