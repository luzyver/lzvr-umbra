<script lang="ts">
	import { onDestroy } from 'svelte';
	import { get, type Status, type ServerConfig } from '#lib/api.ts';
	import { bytes, rate, ago } from '#lib/format.ts';

	import TrafficChart from '#lib/TrafficChart.svelte';
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

	// Diurutkan: online dulu, lalu yang terakhir handshake paling baru.
	const clients = $derived(
		[...(status?.clients ?? [])].filter(c => (!q || `${c.name} ${c.address}`.toLowerCase().includes(q.toLowerCase())) && (filter === 'all' || (filter === 'online' ? c.online && c.enabled : !c.online || !c.enabled))).sort(
			(a, b) =>
				Number(b.online) - Number(a.online) ||
				(b.last_handshake ? Date.parse(b.last_handshake) : 0) - (a.last_handshake ? Date.parse(a.last_handshake) : 0)
		)
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

<dl class="card stats">
	<div><dt>Online</dt><dd>{status?.online ?? '—'}<small>/ {status?.total ?? '—'}</small></dd></div>
	<div><dt>Data masuk</dt><dd>{status ? bytes(status.rx_bytes) : '—'}<small>{live && samples.length ? rate(totalRate.rx) : ''}</small></dd></div>
	<div><dt>Data keluar</dt><dd>{status ? bytes(status.tx_bytes) : '—'}<small>{live && samples.length ? rate(totalRate.tx) : ''}</small></dd></div>
</dl>

<div class="card mt-4"><TrafficChart {samples} {live} /></div>

<section class="mt-8">
	<div class="mb-3 flex flex-wrap items-center gap-3">
		<h2 class="section-title">Perangkat</h2>
		<div class="filter-tabs" aria-label="Filter status">
			{#each [{ v: 'all', l: 'Semua' }, { v: 'online', l: 'Online' }, { v: 'offline', l: 'Offline' }] as tab}
				<button class:selected={filter === tab.v} aria-pressed={filter === tab.v} onclick={() => (filter = tab.v)}>{tab.l}</button>
			{/each}
		</div>
		<input class="input ml-auto w-56! max-w-full" aria-label="Cari perangkat" placeholder="Cari nama atau IP…" bind:value={q} />
	</div>
	<div class="card overflow-x-auto p-0">
		{#if !status}
			<p class="hint px-4 py-8 text-center">{error || 'Memuat perangkat…'}</p>
		{:else if !clients.length}
			<p class="hint px-4 py-8 text-center">
				{status.total ? 'Tidak ada perangkat yang sesuai.' : 'Belum ada perangkat.'}
				{#if !status.total}<a class="font-medium text-[var(--text)] underline" href="/clients?new=1">Tambah perangkat</a>{/if}
			</p>
		{:else}
			<table class="data-table">
				<thead><tr><th>Perangkat</th><th>Status</th><th class="desktop-column">Handshake</th><th class="desktop-column">Unduh / unggah</th></tr></thead>
				<tbody>
					{#each clients as c (c.id)}
						<tr>
							<td><div class="font-medium">{c.name}</div><div class="table-mono">{c.address}</div></td>
							<td><span class="status" class:online={c.online && c.enabled}><i class="status-dot"></i>{!c.enabled ? 'Nonaktif' : c.online ? 'Online' : 'Offline'}</span></td>
							<td class="desktop-column text-[var(--muted)]">{ago(c.last_handshake)}</td>
							<td class="desktop-column table-mono">↓ {bytes(c.tx_bytes)} · ↑ {bytes(c.rx_bytes)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</section>
