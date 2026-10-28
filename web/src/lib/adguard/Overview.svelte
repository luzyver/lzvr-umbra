<script lang="ts">
	import { get, post, put, errMsg, type AdGuardStatus } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Toggle from '#lib/Toggle.svelte';

	let { st, onchange }: { st: NonNullable<AdGuardStatus['status']>; onchange: () => void } = $props();

	// Statistik mentah dari AdGuard Home; top_* berbentuk [{ "domain": jumlah }, ...].
	type Top = Record<string, number>[];
	type Stats = {
		num_dns_queries: number;
		num_blocked_filtering: number;
		num_replaced_safebrowsing: number;
		num_replaced_parental: number;
		avg_processing_time: number;
		top_queried_domains: Top;
		top_blocked_domains: Top;
		top_clients: Top;
	};
	type SafeSearch = { enabled: boolean } & Record<string, boolean>;

	let stats = $state<Stats | null>(null);
	let safeBrowsing = $state(false);
	let parental = $state(false);
	let safeSearch = $state<SafeSearch | null>(null);
	let busy = $state(false);

	async function load() {
		const [s, sb, pc, ss] = await Promise.all([
			get<Stats>('/adguard/stats').catch(() => null),
			get<{ enabled: boolean }>('/adguard/ctl/safebrowsing/status').catch(() => null),
			get<{ enabled: boolean }>('/adguard/ctl/parental/status').catch(() => null),
			get<SafeSearch>('/adguard/ctl/safesearch/status').catch(() => null)
		]);
		stats = s;
		safeBrowsing = !!sb?.enabled;
		parental = !!pc?.enabled;
		safeSearch = ss;
	}
	load();

	async function act(fn: () => Promise<unknown>, ok: string) {
		busy = true;
		try {
			await fn();
			toast(ok);
			await load();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	const setProtection = (v: boolean) =>
		act(async () => {
			await post('/adguard/protection', { enabled: v });
			onchange();
		}, v ? 'Perlindungan AdGuard aktif.' : 'Perlindungan AdGuard dimatikan.');
	const setSafeBrowsing = (v: boolean) =>
		act(() => post(`/adguard/ctl/safebrowsing/${v ? 'enable' : 'disable'}`), 'Safe browsing diperbarui.');
	const setParental = (v: boolean) =>
		act(() => post(`/adguard/ctl/parental/${v ? 'enable' : 'disable'}`), 'Kontrol orang tua diperbarui.');
	const setSafeSearch = (key: string, v: boolean) =>
		act(() => put('/adguard/ctl/safesearch/settings', { ...safeSearch, [key]: v }), 'Safe search diperbarui.');
	function resetStats() {
		if (!confirm('Hapus semua statistik AdGuard?')) return;
		act(() => post('/adguard/ctl/stats_reset'), 'Statistik direset.');
	}

	const engines = $derived(safeSearch ? Object.keys(safeSearch).filter((k) => k !== 'enabled') : []);
	const pct = $derived(
		stats && stats.num_dns_queries > 0
			? ((stats.num_blocked_filtering / stats.num_dns_queries) * 100).toFixed(1)
			: '0'
	);

	function entries(t: Top | undefined, n = 10): [string, number][] {
		return (t ?? []).slice(0, n).map((o) => Object.entries(o)[0] as [string, number]);
	}
</script>

<div class="mb-4 grid gap-4 lg:grid-cols-3">
	<div class="card space-y-3">
		<div class="flex items-center justify-between">
			<div>
				<p class="font-medium">Perlindungan</p>
				<p class="hint">Blokir iklan & pelacak untuk klien yang mengaktifkannya.</p>
			</div>
			<Toggle checked={st.protection_enabled} disabled={busy} label="Perlindungan" onchange={setProtection} />
		</div>
		<dl class="grid grid-cols-2 gap-y-1 text-sm">
			<dt class="text-zinc-500">Status</dt>
			<dd>{st.running ? 'berjalan' : 'berhenti'}</dd>
			<dt class="text-zinc-500">Versi</dt>
			<dd class="font-mono text-xs">{st.version}</dd>
			<dt class="text-zinc-500">Alamat DNS</dt>
			<dd class="font-mono text-xs">{st.dns_addresses.join(', ')}:{st.dns_port}</dd>
		</dl>
	</div>

	<div class="card space-y-3 text-sm">
		<p class="font-medium">Perlindungan tambahan</p>
		<div class="flex items-center justify-between gap-2">
			<span>Safe browsing <span class="hint block">Blokir situs phishing & malware.</span></span>
			<Toggle checked={safeBrowsing} disabled={busy} label="Safe browsing" onchange={setSafeBrowsing} />
		</div>
		<div class="flex items-center justify-between gap-2">
			<span>Kontrol orang tua <span class="hint block">Blokir situs dewasa.</span></span>
			<Toggle checked={parental} disabled={busy} label="Kontrol orang tua" onchange={setParental} />
		</div>
		{#if safeSearch}
			<div class="flex items-center justify-between gap-2">
				<span>Safe search <span class="hint block">Paksa mode aman di mesin pencari.</span></span>
				<Toggle checked={safeSearch.enabled} disabled={busy} label="Safe search" onchange={(v) => setSafeSearch('enabled', v)} />
			</div>
			{#if safeSearch.enabled}
				<div class="flex flex-wrap gap-x-4 gap-y-1">
					{#each engines as k (k)}
						<label class="flex items-center gap-1 text-xs">
							<input type="checkbox" checked={safeSearch[k]} disabled={busy} onchange={(e) => setSafeSearch(k, e.currentTarget.checked)} />
							{k}
						</label>
					{/each}
				</div>
			{/if}
		{/if}
		<p class="hint">Berlaku untuk semua klien, kecuali yang punya pengaturan sendiri di tab Klien.</p>
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
			<div class="col-span-2 flex items-center justify-between gap-2">
				<p class="hint">Rata-rata proses: {(stats.avg_processing_time * 1000).toFixed(1)} ms</p>
				<button class="btn-ghost text-xs" disabled={busy} onclick={resetStats}>Reset statistik</button>
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
