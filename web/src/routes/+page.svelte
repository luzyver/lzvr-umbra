<script lang="ts">
	import { onDestroy } from 'svelte';
	import { get, type Status, type ServerConfig } from '#lib/api.ts';
	import { bytes, rate, ago } from '#lib/format.ts';

	import Icon from '#lib/Icon.svelte';
	import Logo from '#lib/Logo.svelte';
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
<div class="page-heading"><div><div class="overline">YOUR PRIVATE NETWORK</div><h1>Jaringan dalam kendalimu.</h1><p>Satu ruang untuk semua koneksi pribadimu.</p></div><a class="btn-primary" href="/clients?new=1"><Icon name="plus" size={16}/>Tambah perangkat</a></div>
<div class="health-strip" class:pending={!live} class:error={!!(error || status?.error)} role="status"><Icon name="shield" size={16}/><strong>{error || status?.error || (live ? (status?.initialized ? 'Jaringan siap digunakan' : 'Jaringan belum diinisialisasi') : 'Menghubungkan pembaruan langsung…')}</strong><span class="health-caption">{status?.interface ?? 'WireGuard'} · Private network</span><span class="health-indicator"><i class="status-dot"></i>{live ? 'LIVE' : 'TERPUTUS'}</span></div>
<div class="metric-grid">
 <div class="card metric-card"><div class="metric-label">Perangkat online<Icon name="laptop" size={17}/></div><div class="metric-value">{status?.online ?? '—'}<small>/ {status?.total ?? '—'}</small></div><div class="metric-meta"><strong>{live ? 'Status langsung' : 'Status terakhir'}</strong><span>perangkat terhubung</span></div></div>
 {#each [{key:'rx_bytes' as const,dir:'rx' as const,label:'Data masuk'},{key:'tx_bytes' as const,dir:'tx' as const,label:'Data keluar'}] as m}
 <div class="card metric-card"><div class="metric-label">{m.label}<Icon name="arrow" size={17}/></div><div class="metric-value">{status ? bytes(status[m.key]).split(' ')[0] : '—'}<small>{status ? bytes(status[m.key]).split(' ')[1] : ''}</small></div><div class="metric-meta" class:purple={m.dir === 'tx'}><strong>{live && samples.length ? rate(totalRate[m.dir]) : '—'}</strong><span>melalui server</span></div></div>
 {/each}
</div>
<div class="middle-grid"><section class="card traffic-card"><TrafficChart {samples} {live}/></section><section class="card gateway-card"><div class="panel-heading"><h2>Gateway pribadi</h2><Icon name="server" size={17}/></div><div class="gateway-visual" aria-hidden="true"><div class="orbit"></div><div class="orbit two"></div><div class="orbit three"></div><div class="gateway-box"><Logo wordmark={false}/></div></div><div class="gateway-name">WireGuard <span>{server?.interface ?? '—'}</span></div>{#if serverError}<p class="hint px-5 py-3">{serverError}</p>{:else}<dl class="gateway-data"><div><dt>Endpoint</dt><dd>{server?.endpoint_host || '—'}</dd></div><div><dt>Port</dt><dd>{server?.endpoint_port ?? '—'}</dd></div><div><dt>Jaringan</dt><dd>{server?.address ?? '—'}</dd></div></dl>{/if}</section></div>
<section class="card table-panel"><div class="panel-heading"><div><h2>Perangkatmu</h2><p>Semua koneksi, dalam satu pandangan.</p></div><a class="text-link" href="/clients">Kelola perangkat <Icon name="arrow" size={14}/></a></div><div class="table-toolbar"><div class="filter-tabs" aria-label="Filter status">{#each [{v:'all',l:'Semua'},{v:'online',l:'Online'},{v:'offline',l:'Offline'}] as tab}<button class:selected={filter===tab.v} aria-pressed={filter===tab.v} onclick={() => filter=tab.v}>{tab.l}</button>{/each}</div><label class="device-search"><Icon name="search" size={14}/><input aria-label="Cari perangkat" placeholder="Cari nama atau alamat IP…" bind:value={q}/></label></div>
 {#if !status}<div class="empty-state">{error || 'Memuat perangkat…'}</div>{:else if !clients.length}<div class="empty-state">{status.total ? 'Tidak ada perangkat yang sesuai.' : 'Jaringanmu siap untuk perangkat pertama.'}{#if !status.total} <a href="/clients?new=1">Tambah perangkat</a>{/if}</div>{:else}<div class="overflow-x-auto"><table class="data-table"><thead><tr><th>Perangkat</th><th>Status</th><th class="desktop-column">Handshake</th><th class="desktop-column">Unduh / unggah</th></tr></thead><tbody>{#each clients as c (c.id)}<tr><td><div class="device-cell"><span class="device-icon"><Icon name="laptop" size={16}/></span><div><strong>{c.name}</strong><small class="table-mono">{c.address}</small></div></div></td><td><span class="online-status" class:off={!c.online || !c.enabled}><i class="status-dot"></i>{!c.enabled ? 'Nonaktif' : c.online ? 'Online' : 'Offline'}</span></td><td class="desktop-column table-muted">{ago(c.last_handshake)}</td><td class="desktop-column table-mono">↓ {bytes(c.tx_bytes)} · ↑ {bytes(c.rx_bytes)}</td></tr>{/each}</tbody></table></div>{/if}<div class="table-end"><span>{clients.length} dari {status?.total ?? '—'} perangkat</span><span>{live ? 'Diperbarui secara langsung' : 'Menampilkan data terakhir'}</span></div></section>
