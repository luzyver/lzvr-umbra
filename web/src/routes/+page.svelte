<script lang="ts">
	import { onDestroy } from 'svelte';
	import { get, type Status, type ServerConfig } from '#lib/api.ts';
	import { bytes, rate } from '#lib/format.ts';

	import TrafficChart from '#lib/TrafficChart.svelte';
	import KeyRack from '#lib/KeyRack.svelte';
	import { sampleTraffic, appendSample, type TrafficSample } from '#lib/traffic.ts';
	let server = $state<ServerConfig|null>(null);
	let serverError = $state('');
	get<ServerConfig>('/server').then(s => server=s).catch(() => serverError='Konfigurasi server belum tersedia.');
	let samples = $state<TrafficSample[]>([]);
	let previous: Status|null = null;
	let error = $state('');
	let q = $state('');
	let filter = $state('all');
	let status = $state<Status | null>(null);
	let live = $state(false);

	function update(s: Status) {
		const t = new Date(s.time).getTime();
        if (!Number.isFinite(t) || (status && t <= Date.parse(status.time))) return;
        const sample = sampleTraffic(previous,s);
        if (sample) samples = appendSample(samples,sample);
        if (!previous) samples = [];
        previous = s;
        error = '';
		status = s;
	}

	get<Status>('/status').then(update).catch(() => error='Status jaringan belum dapat dimuat.');

	const es = new EventSource('/api/events');
	es.addEventListener('status', (e) => {
		try {
			update(JSON.parse((e as MessageEvent).data));
			live = true;
		} catch {
			/* abaikan event rusak */
		}
	});
	es.onerror = () => { live = false; previous = null; };
	onDestroy(() => es.close());

	// Online dulu, lalu menurut nomor IP agar letak label di papan tetap dan hanya bergeser saat status berubah.
	const ipKey = (a: string) => a.split('/')[0].split(/[.:]/).map((p) => p.padStart(5, '0')).join('.');
	const clients = $derived(
		[...(status?.clients ?? [])]
			.filter((c) => (!q || `${c.name} ${c.address}`.toLowerCase().includes(q.toLowerCase())) && (filter === 'all' || (filter === 'online' ? c.online && c.enabled : !c.online || !c.enabled)))
			.sort((a, b) => Number(b.online && b.enabled) - Number(a.online && a.enabled) || ipKey(a.address).localeCompare(ipKey(b.address)))
	);
	const totalRate = $derived(samples.at(-1) ?? { rx: 0, tx: 0 });

</script>

<svelte:head><title>Ringkasan · Umbra</title></svelte:head>

<div class="page-heading">
	<div>
		<h1>Ringkasan</h1>
		{#if serverError}<p>{serverError}</p>{:else if server}<p class="table-mono">{server.interface} · {server.endpoint_host || '—'}:{server.endpoint_port} · {server.address}</p>{/if}
	</div>
	<a class="btn-primary ml-auto" href="/clients?new=1">Tambah perangkat</a>
</div>

{#if error || status?.error}<div class="card mb-4 py-3 text-sm text-red-600" role="alert">{error || status?.error}</div>{/if}

<section class="card pulse" aria-label="Status server">
	<dl class="pulse-stats">
		<div><dt><span class="status" class:online={!!status?.online}><i class="status-dot"></i></span>Online</dt><dd>{status?.online ?? '—'}<small>dari {status?.total ?? '—'} perangkat</small></dd></div>
		<div><dt><i class="legend"></i>Data masuk</dt><dd>{status ? bytes(status.rx_bytes) : '—'}<small>{live && samples.length ? rate(totalRate.rx) : ''}</small></dd></div>
		<div><dt><i class="legend tx"></i>Data keluar</dt><dd>{status ? bytes(status.tx_bytes) : '—'}<small>{live && samples.length ? rate(totalRate.tx) : ''}</small></dd></div>
	</dl>
	<div class="pulse-chart"><TrafficChart {samples} {live} /></div>
</section>

<section class="mt-10">
	<div class="mb-4 flex flex-wrap items-center gap-3">
		<h2 class="section-title">Perangkat</h2>
		<div class="filter-tabs" aria-label="Filter status">
			{#each [{ v: 'all', l: 'Semua' }, { v: 'online', l: 'Online' }, { v: 'offline', l: 'Offline' }] as tab}
				<button class:selected={filter === tab.v} aria-pressed={filter === tab.v} onclick={() => (filter = tab.v)}>{tab.l}</button>
			{/each}
		</div>
		<input class="input ml-auto w-56! max-w-full" aria-label="Cari perangkat" placeholder="Cari nama atau IP…" bind:value={q} />
	</div>
	{#if !status}
		<div class="rack-empty">{error || 'Memuat perangkat…'}</div>
	{:else if !status.total}
		<div class="rack-empty">Belum ada perangkat. <a class="font-semibold text-[var(--text)] underline" href="/clients?new=1">Tambah perangkat</a></div>
	{:else}
		<KeyRack {clients} empty="Tidak ada perangkat yang sesuai." />
	{/if}
</section>
