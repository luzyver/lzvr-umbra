<script lang="ts">
	// QR + tombol unduh profil klien. QR diambil sebagai blob karena endpoint butuh sesi.
	import type { Client } from './api';

	let { client }: { client: Client } = $props();

	let qr = $state('');
	let failed = $state(false);

	$effect(() => {
		const id = client.id;
		const stamp = client.updated_at;
		let url = '';
		failed = false;
		fetch(`/api/clients/${id}/qr.png?v=${encodeURIComponent(stamp)}`, { credentials: 'same-origin' })
			.then((r) => (r.ok ? r.blob() : Promise.reject()))
			.then((b) => {
				url = URL.createObjectURL(b);
				qr = url;
			})
			.catch(() => (failed = true));
		return () => {
			if (url) URL.revokeObjectURL(url);
		};
	});
</script>

<div class="flex flex-col items-center gap-3">
	{#if qr}
		<img src={qr} alt="QR profil {client.name}" class="w-64 rounded-lg bg-white p-2" />
	{:else if failed}
		<p class="text-sm text-red-600">QR gagal dimuat.</p>
	{:else}
		<div class="h-64 w-64 animate-pulse rounded-lg bg-zinc-200 dark:bg-zinc-800"></div>
	{/if}
	<p class="hint text-center">Pindai dengan aplikasi WireGuard di HP (Tambah tunnel → Pindai QR).</p>
	<a class="btn-ghost" href="/api/clients/{client.id}/config" download>Unduh file .conf</a>
	<a class="btn-ghost" href="/activity?client={client.id}">Aktivitas jaringan</a>
</div>
