<script lang="ts">
	import { get, put, errMsg } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import ServicePicker from './ServicePicker.svelte';

	let ids = $state<string[]>([]);
	let schedule = $state<unknown>(undefined);
	let loaded = $state(false);
	let busy = $state(false);

	get<{ ids: string[] | null; schedule?: unknown }>('/adguard/ctl/blocked_services/get')
		.then((r) => {
			ids = r.ids ?? [];
			schedule = r.schedule;
			loaded = true;
		})
		.catch((e) => toast(errMsg(e), 'error'));

	async function save() {
		busy = true;
		try {
			// Jadwal yang sudah ada dikirim balik apa adanya agar tidak terhapus.
			await put('/adguard/ctl/blocked_services/update', { ids, schedule });
			toast('Layanan diblokir diperbarui.');
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

<div class="card space-y-3">
	<div>
		<p class="font-medium">Blokir layanan</p>
		<p class="hint">
			Berlaku untuk semua klien yang memakai AdGuard, kecuali klien dengan daftar layanan sendiri
			(atur di tab Klien).
		</p>
	</div>
	{#if loaded}
		<ServicePicker bind:selected={ids} disabled={busy} />
		<div class="flex justify-end">
			<button class="btn-primary" disabled={busy} onclick={save}>Simpan</button>
		</div>
	{:else}
		<p class="hint">Memuat…</p>
	{/if}
</div>
