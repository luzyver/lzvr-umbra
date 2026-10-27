<script lang="ts">
	import { get, post, errMsg, ApiError, type AdGuardStatus } from '#lib/api.ts';
	import { toast, warnAll } from '#lib/toast.svelte.ts';
	import Toggle from '#lib/Toggle.svelte';

	// Statistik mentah dari AdGuard Home; top_* berbentuk [{ "domain": jumlah }, ...].
	type Top = Record<string, number>[];
	type Stats = {
		num_dns_queries: number;
		num_blocked_filtering: number;
		avg_processing_time: number;
		top_queried_domains: Top;
		top_blocked_domains: Top;
		top_clients: Top;
	};

	let st = $state<AdGuardStatus | null>(null);
	let stats = $state<Stats | null>(null);
	let error = $state('');
	let busy = $state(false);

	async function load() {
		error = '';
		try {
			st = await get<AdGuardStatus>('/adguard/status');
			if (st.configured && st.reachable) {
				stats = await get<Stats>('/adguard/stats').catch(() => null);
			}
		} catch (e) {
			if (e instanceof ApiError && e.status === 503) st = { configured: false };
			else error = errMsg(e);
		}
	}
	load();

	async function setProtection(v: boolean) {
		busy = true;
		try {
			await post('/adguard/protection', { enabled: v });
			toast(v ? 'Perlindungan AdGuard aktif.' : 'Perlindungan AdGuard dimatikan.');
			await load();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	async function sync() {
		busy = true;
		try {
			const r = await post<{ synced: number; removed: number; warnings: string[] }>('/adguard/sync');
			warnAll(r.warnings);
			toast(`Sinkron selesai: ${r.synced} klien diperbarui, ${r.removed} dihapus.`);
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	const pct = $derived(
		stats && stats.num_dns_queries > 0
			? ((stats.num_blocked_filtering / stats.num_dns_queries) * 100).toFixed(1)
			: '0'
	);

	function entries(t: Top | undefined, n = 10): [string, number][] {
		return (t ?? []).slice(0, n).map((o) => Object.entries(o)[0] as [string, number]);
	}
</script>

<svelte:head><title>AdGuard · Umbra</title></svelte:head>

<div class="mb-4 flex items-center gap-3">
	<h1 class="text-xl font-semibold">AdGuard</h1>
	{#if st?.configured}
		<button class="btn-ghost ml-auto" disabled={busy} onclick={load}>Muat ulang</button>
		<button class="btn-primary" disabled={busy || !st.reachable} onclick={sync}>Sinkronkan klien</button>
	{/if}
</div>

{#if error}
	<div class="card text-sm text-red-600">{error}</div>
{:else if !st}
	<p class="text-sm text-zinc-500">Memuat…</p>
{:else if !st.configured}
	<div class="card space-y-3 text-sm">
		<p class="font-medium">AdGuard Home belum dihubungkan.</p>
		<p class="text-zinc-600 dark:text-zinc-400">
			Pasang AdGuard Home di server ini (dengarkan DNS hanya di IP WireGuard, misalnya
			<span class="font-mono">10.8.0.1:53</span>, dan web UI di <span class="font-mono">127.0.0.1:3000</span>),
			lalu isi variabel berikut di <span class="font-mono">/etc/umbra/umbra.env</span> dan restart panel:
		</p>
		<pre class="overflow-x-auto rounded-lg bg-zinc-100 p-3 font-mono text-xs dark:bg-zinc-950">UMBRA_ADGUARD_URL=http://127.0.0.1:3000
UMBRA_ADGUARD_USER=admin
UMBRA_ADGUARD_PASS=kata-sandi-adguard</pre>
		<p class="hint">Setelah itu atur DNS bawaan klien ke IP server WireGuard (misalnya 10.8.0.1) di Pengaturan.</p>
	</div>
{:else if !st.reachable}
	<div class="card space-y-2 text-sm">
		<p class="font-medium text-red-600">AdGuard Home tidak bisa dihubungi.</p>
		{#if st.error}<p class="font-mono text-xs text-zinc-500">{st.error}</p>{/if}
		<p class="hint">Periksa apakah service AdGuard berjalan dan alamat/kata sandinya benar.</p>
	</div>
{:else if st.status}
	<div class="mb-6 grid gap-4 sm:grid-cols-2">
		<div class="card space-y-3">
			<div class="flex items-center justify-between">
				<div>
					<p class="font-medium">Perlindungan</p>
					<p class="hint">Blokir iklan & pelacak untuk klien yang mengaktifkannya.</p>
				</div>
				<Toggle
					checked={st.status.protection_enabled}
					disabled={busy}
					label="Perlindungan"
					onchange={setProtection}
				/>
			</div>
			<dl class="grid grid-cols-2 gap-y-1 text-sm">
				<dt class="text-zinc-500">Status</dt>
				<dd>{st.status.running ? 'berjalan' : 'berhenti'}</dd>
				<dt class="text-zinc-500">Versi</dt>
				<dd class="font-mono text-xs">{st.status.version}</dd>
				<dt class="text-zinc-500">Alamat DNS</dt>
				<dd class="font-mono text-xs">{st.status.dns_addresses.join(', ')}:{st.status.dns_port}</dd>
			</dl>
		</div>
		{#if stats}
			<div class="card grid grid-cols-2 gap-4">
				<div>
					<p class="hint">Query DNS</p>
					<p class="mt-1 text-2xl font-semibold tabular-nums">{stats.num_dns_queries.toLocaleString('id-ID')}</p>
				</div>
				<div>
					<p class="hint">Diblokir</p>
					<p class="mt-1 text-2xl font-semibold tabular-nums">{stats.num_blocked_filtering.toLocaleString('id-ID')}</p>
					<p class="hint">{pct}%</p>
				</div>
				<div class="col-span-2">
					<p class="hint">Rata-rata waktu proses: {(stats.avg_processing_time * 1000).toFixed(1)} ms</p>
				</div>
			</div>
		{/if}
	</div>

	{#if stats}
		<div class="grid gap-4 md:grid-cols-3">
			{#each [['Domain terbanyak', stats.top_queried_domains], ['Paling sering diblokir', stats.top_blocked_domains], ['Klien teraktif', stats.top_clients]] as [title, list] (title)}
				<div class="card">
					<p class="mb-2 font-medium">{title}</p>
					<ul class="space-y-1 text-sm">
						{#each entries(list as Top) as [k, n] (k)}
							<li class="flex justify-between gap-2">
								<span class="truncate font-mono text-xs" title={k}>{k}</span>
								<span class="tabular-nums text-zinc-500">{n}</span>
							</li>
						{:else}
							<li class="hint">Belum ada data.</li>
						{/each}
					</ul>
				</div>
			{/each}
		</div>
	{/if}
{/if}
