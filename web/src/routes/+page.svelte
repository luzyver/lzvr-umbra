<script lang="ts">
	import { onMount } from 'svelte';
	import { get, type ServerConfig, type AdGuardStatus } from '#lib/api.ts';
	import { bytes, rate } from '#lib/format.ts';
	import { session } from '#lib/session.svelte.ts';
	import { createLiveStatus } from '#lib/live-status.svelte.ts';
	import TrafficChart from '#lib/TrafficChart.svelte';
	import DeviceList from '#lib/DeviceList.svelte';
	import DeviceDetails from '#lib/DeviceDetails.svelte';
	import Icon from '#lib/Icon.svelte';
	import OverviewContext from '#lib/OverviewContext.svelte';
	import '#lib/overview.css';

	const feed = createLiveStatus();
	let server = $state<ServerConfig | null>(null);
	let serverError = $state('');
	let adguard = $state<AdGuardStatus | null>(null);
	let adguardError = $state(false);
	let selected = $state<number | null>(null);
	let detailOpen = $state(false);
	const device = $derived(feed.status?.clients.find(c => c.id === selected) ?? null);
	const latest = $derived(feed.samples.at(-1));
	const now = $derived(feed.status ? Date.parse(feed.status.time) : Date.now());
	onMount(() => {
		let active = true;
		get<ServerConfig>('/server').then(s => { if (active) server = s; }).catch(() => { if (active) serverError = 'Konfigurasi server belum tersedia.'; });
		const refreshDns = () => {
			if (!session.me?.adguard) return;
			get<AdGuardStatus>('/adguard/status').then(s => { if (active) { adguard = s; adguardError = false; } }).catch(() => { if (active) adguardError = true; });
		};
		refreshDns();
		const timer = setInterval(refreshDns, 30000);
		return () => { active = false; clearInterval(timer); };
	});
	const dnsHealthy = $derived(!adguardError && !!adguard?.reachable && !!adguard.status?.running && !!adguard.status?.protection_enabled);
	const dnsLabel = $derived(adguardError ? 'Tidak terjangkau' : !adguard ? 'Memeriksa…' : !adguard.configured ? 'Belum diatur' : !adguard.reachable ? 'Tidak terjangkau' : !adguard.status?.running ? 'DNS berhenti' : adguard.status.protection_enabled ? 'Perlindungan aktif' : 'Perlindungan nonaktif');
</script>

<svelte:head><title>Ringkasan · Umbra</title></svelte:head>

<div class="page-heading"><h1>Ringkasan</h1><a class="btn-primary ml-auto" href="/clients?new=1"><Icon name="plus" size={16} />Tambah perangkat</a></div>

<div class="overview-summary" aria-label="Status layanan">
	<a href="/settings" class="service-item"><Icon name="server" size={22} /><div><strong>WireGuard</strong><span class="status" class:online={feed.current}><i class="status-dot"></i>{feed.error ? 'Perlu diperiksa' : feed.current ? 'Tersedia' : 'Memeriksa…'}</span></div></a>
	{#if session.me?.adguard}<a href="/adguard" class="service-item"><Icon name="shield" size={22} /><div><strong>AdGuard</strong><span class="status" class:online={dnsHealthy}><i class="status-dot"></i>{dnsLabel}</span></div></a>{/if}
	<div class="summary-devices"><Icon name="laptop" size={22} /><div><span>Perangkat online</span><strong>{feed.current ? feed.status?.online : '—'} <small>/ {feed.status?.total ?? '—'}</small></strong></div></div>
</div>

{#if feed.error}<div class="inline-notice" role="status">{feed.error}<span>Status dan trafik perangkat ditampilkan kembali setelah koneksi pulih.</span></div>{/if}

<div class="monitoring-grid">
	<section class="monitoring-main" aria-label="Trafik server">
		<dl class="monitoring-rates">
			<div><dt><Icon name="down" size={16} />Masuk ke server</dt><dd>{feed.live && feed.current && latest ? rate(latest.rx) : '—'}</dd></div>
			<div><dt><Icon name="up" size={16} />Keluar dari server</dt><dd>{feed.live && feed.current && latest ? rate(latest.tx) : '—'}</dd></div>
		</dl>
		<TrafficChart samples={feed.samples} live={feed.live && feed.current} />
		<div class="monitoring-totals"><span>Total masuk <strong>{feed.current ? bytes(feed.status?.rx_bytes ?? 0) : '—'}</strong></span><span>Total keluar <strong>{feed.current ? bytes(feed.status?.tx_bytes ?? 0) : '—'}</strong></span></div>
	</section>
	<OverviewContext {server} {serverError} adguard={adguardError ? null : adguard} {dnsLabel} showDns={!!session.me?.adguard} />
</div>

<DeviceList devices={feed.status?.clients ?? []} current={feed.current} loading={!feed.status && !feed.error} unavailable={!!feed.error && !feed.status?.clients.length} selected={detailOpen ? selected ?? undefined : undefined} {now} onselect={id => { selected = id; detailOpen = true; }} />
<DeviceDetails bind:open={detailOpen} {device} current={feed.current} {now} />
