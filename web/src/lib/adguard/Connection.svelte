<script lang="ts">
	import { get, post, del, errMsg, type AdGuardConn } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import ConnForm from './ConnForm.svelte';

	let { onchange }: { onchange: () => void } = $props();

	let conn = $state<AdGuardConn | null>(null);
	let editing = $state(false);
	let busy = $state(false);

	get<AdGuardConn>('/adguard/connection')
		.then((r) => (conn = r))
		.catch((e) => toast(errMsg(e), 'error'));

	const sources = { db: 'disimpan di panel', env: 'dari umbra.env', none: '-' };

	async function disconnect() {
		if (!confirm('Putuskan panel dari AdGuard? AdGuard Home tetap berjalan, DNS klien tidak diubah.')) return;
		busy = true;
		try {
			const r = await del<{ enabled: boolean }>('/adguard/connection');
			toast(r?.enabled ? 'Koneksi panel dihapus, tapi umbra.env masih berisi AdGuard.' : 'AdGuard diputuskan.');
			onchange();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	async function uninstall() {
		if (!confirm('Copot AdGuard Home dari server? DNS klien dikembalikan ke DNS default server.')) return;
		busy = true;
		try {
			await post('/adguard/uninstall');
			toast('Mencopot AdGuard…');
			onchange();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

{#if conn}
	<div class="grid gap-4 md:grid-cols-2">
		<div class="card space-y-3 text-sm">
			<p class="font-medium">Koneksi</p>
			<dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
				<dt class="text-zinc-500">Alamat</dt>
				<dd class="font-mono text-xs">{conn.url}</dd>
				<dt class="text-zinc-500">Username</dt>
				<dd>{conn.user || '-'}</dd>
				<dt class="text-zinc-500">Sumber</dt>
				<dd>{sources[conn.source]}</dd>
				<dt class="text-zinc-500">Dipasang panel</dt>
				<dd>{conn.managed ? 'ya' : 'tidak'}</dd>
			</dl>
			{#if editing}
				<ConnForm url={conn.url} user={conn.user} hasPassword={conn.has_password} onsaved={onchange} />
			{:else}
				<div class="flex flex-wrap gap-2">
					<button class="btn-ghost" onclick={() => (editing = true)}>Ubah koneksi</button>
					{#if conn.source === 'db'}
						<button class="btn-ghost" disabled={busy} onclick={disconnect}>Putuskan</button>
					{/if}
				</div>
			{/if}
			{#if conn.source === 'env'}
				<p class="hint">Menyimpan dari sini akan menimpa nilai di umbra.env tanpa restart.</p>
			{/if}
		</div>

		{#if conn.managed}
			<div class="card space-y-3 text-sm">
				<p class="font-medium">Copot AdGuard Home</p>
				<p class="hint">
					Menghentikan dan menghapus AdGuard Home yang dipasang panel, lalu mengembalikan DNS klien
					ke DNS default server.
				</p>
				<button class="btn-danger" disabled={busy} onclick={uninstall}>Copot AdGuard</button>
			</div>
		{/if}
	</div>
{:else}
	<p class="hint">Memuat…</p>
{/if}
