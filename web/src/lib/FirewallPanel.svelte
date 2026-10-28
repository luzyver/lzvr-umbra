<script lang="ts">
	// Firewall dengan masa uji 60 detik: aturan otomatis dibatalkan bila tidak dikonfirmasi.
	import { onDestroy } from 'svelte';
	import { get, post, errMsg, type FirewallInfo } from './api';
	import { toast } from './toast.svelte';

	let { onconfirmed }: { onconfirmed?: () => void } = $props();

	let info = $state<FirewallInfo | null>(null);
	let mss = $state(true);
	let busy = $state(false);
	let error = $state('');
	let now = $state(Date.now());
	let showPreview = $state(false);

	const timer = setInterval(() => {
		now = Date.now();
		if (info?.pending && new Date(info.pending.expires).getTime() <= now) load();
	}, 1000);
	onDestroy(() => clearInterval(timer));

	const left = $derived(
		info?.pending ? Math.max(0, Math.ceil((new Date(info.pending.expires).getTime() - now) / 1000)) : 0
	);

	async function load() {
		try {
			info = await get<FirewallInfo>('/firewall');
			if (!info.pending) mss = info.state.applied ? info.state.mss_clamp : true;
		} catch (e) {
			error = errMsg(e);
		}
	}
	load();

	async function act(fn: () => Promise<unknown>, ok?: string) {
		busy = true;
		error = '';
		try {
			await fn();
			if (ok) toast(ok);
			await load();
		} catch (e) {
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}

	const apply = () => act(() => post('/firewall/apply', { mss_clamp: mss }));
	const confirm = () =>
		act(async () => {
			await post('/firewall/confirm');
			onconfirmed?.();
		}, 'Firewall aktif dan tersimpan.');
	const remove = () => act(() => post('/firewall/remove'), 'Aturan firewall panel dihapus.');
</script>

{#if !info}
	<p class="text-sm text-zinc-500">Memuat…</p>
{:else}
	<div class="space-y-4">
		<p class="text-sm text-zinc-600 dark:text-zinc-400">
			Panel membuat tabel nftables sendiri (<span class="font-mono">inet umbra</span>) untuk NAT dan
			meneruskan trafik <span class="font-mono">{info.rules.wg_interface}</span> →
			<span class="font-mono">{info.rules.out_interface}</span> (subnet
			<span class="font-mono">{info.rules.subnet}</span>). Aturan lain di server tidak disentuh.
		</p>

		<div class="flex items-center gap-2 text-sm">
			Status:
			{#if info.pending}
				<span class="badge bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200">masa uji</span>
			{:else if info.state.applied}
				<span class="badge border border-[var(--border)] text-[var(--muted)]">aktif</span>
			{:else}
				<span class="badge bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300">belum dipasang</span>
			{/if}
		</div>

		{#if info.pending}
			<div class="rounded-lg border border-amber-300 bg-amber-50 p-4 dark:border-amber-800 dark:bg-amber-950/40">
				<p class="font-medium">Aturan sedang diuji — sisa {left} detik</p>
				<p class="hint mt-1">
					Coba sambungkan klien atau pastikan panel ini masih bisa dibuka. Bila tidak dikonfirmasi,
					aturan dibatalkan otomatis supaya Anda tidak terkunci.
				</p>
				<div class="mt-3 flex gap-2">
					<button class="btn-primary" disabled={busy} onclick={confirm}>Semua aman, konfirmasi</button>
					<button class="btn-ghost" disabled={busy} onclick={remove}>Batalkan</button>
				</div>
			</div>
		{:else}
			<label class="flex items-start gap-2 text-sm">
				<input type="checkbox" class="mt-0.5" bind:checked={mss} />
				<span>
					MSS clamping
					<span class="hint block">Disarankan. Mencegah situs tertentu macet karena ukuran paket.</span>
				</span>
			</label>
			<div class="flex flex-wrap gap-2">
				<button class="btn-primary" disabled={busy} onclick={apply}>
					{info.state.applied ? 'Terapkan ulang (uji 60 detik)' : 'Pasang firewall (uji 60 detik)'}
				</button>
				{#if info.state.applied}
					<button class="btn-ghost" disabled={busy} onclick={remove}>Hapus aturan</button>
				{/if}
				<button class="btn-ghost" type="button" onclick={() => (showPreview = !showPreview)}>
					{showPreview ? 'Sembunyikan' : 'Lihat'} aturan
				</button>
			</div>
		{/if}

		{#if showPreview}
			{#if info.preview_error}
				<p class="text-sm text-red-600">{info.preview_error}</p>
			{:else}
				<pre class="overflow-x-auto rounded-lg bg-zinc-100 p-3 font-mono text-xs dark:bg-zinc-950">{info.preview}</pre>
			{/if}
		{/if}

		{#if error}<p class="text-sm text-red-600">{error}</p>{/if}
	</div>
{/if}
