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
	import PortPanel from '#lib/PortPanel.svelte';
	import '#lib/overview.css';

	const feed = createLiveStatus();
	let server = $state<ServerConfig | null>(null);
	let serverError = $state('');
	let adguard = $state<AdGuardStatus | null>(null);
	let adguardError = $state(false);
	let selected = $state<number | null>(null);
	let detailOpen = $state(false);
	const devices = $derived(feed.status?.clients ?? []);
	const device = $derived(devices.find(c => c.id === selected) ?? null);
	const latest = $derived(feed.samples.at(-1));
	const streaming = $derived(feed.live && feed.current);
	// "1.7 MB/s" is split so the unit can be set smaller than the number.
	const rxRate = $derived(streaming && latest ? rate(latest.rx).split(' ') : ['—']);
	const txRate = $derived(streaming && latest ? rate(latest.tx).split(' ') : ['—']);
	const now = $derived(feed.status ? Date.parse(feed.status.time) : Date.now());
	const open = (id: number) => { selected = id; detailOpen = true; };
	onMount(() => {
		let active = true;
		get<ServerConfig>('/server').then(s => { if (active) server = s; }).catch(() => { if (active) serverError = 'Server configuration is not available yet.'; });
		const refreshDns = () => {
			if (!session.me?.adguard) return;
			get<AdGuardStatus>('/adguard/status').then(s => { if (active) { adguard = s; adguardError = false; } }).catch(() => { if (active) adguardError = true; });
		};
		refreshDns();
		const timer = setInterval(refreshDns, 30000);
		return () => { active = false; clearInterval(timer); };
	});
	const dnsHealthy = $derived(!adguardError && !!adguard?.reachable && !!adguard.status?.running && !!adguard.status?.protection_enabled);
	const dnsLabel = $derived(adguardError ? 'Unreachable' : !adguard ? 'Checking…' : !adguard.configured ? 'Not set up' : !adguard.reachable ? 'Unreachable' : !adguard.status?.running ? 'DNS stopped' : adguard.status.protection_enabled ? 'Protection enabled' : 'Protection disabled');
</script>

<svelte:head><title>Overview · Umbra</title></svelte:head>

<div class="page-heading">
	<h1>Overview</h1>
	<div class="service-inline" aria-label="Service status">
		<a href="/settings" class:ok={feed.current}><i></i><strong>WireGuard</strong>{feed.error ? 'Needs attention' : feed.current ? 'Available' : 'Checking…'}</a>
		{#if session.me?.adguard}<a href="/adguard" class:ok={dnsHealthy}><i></i><strong>AdGuard</strong>{dnsLabel}</a>{/if}
	</div>
	<a class="btn-primary" href="/clients?new=1"><Icon name="plus" size={16} />Add device</a>
</div>

{#if feed.error}<div class="inline-notice" role="status">{feed.error}<span>Device status and traffic will show again once the connection recovers.</span></div>{/if}

{#if server}<PortPanel address={server.address} name={server.interface} {devices} known={feed.inventory} current={feed.current} rates={feed.rates} selected={detailOpen ? selected ?? undefined : undefined} onselect={open} />{/if}

<div class="monitoring-grid">
	<section class="monitoring-main" aria-label="Server traffic">
		<dl class="monitoring-rates">
			<div><dt>Into server</dt><dd>{rxRate[0]}{#if rxRate[1]}<small>{rxRate[1]}</small>{/if}</dd></div>
			<div><dt>Out of server</dt><dd>{txRate[0]}{#if txRate[1]}<small>{txRate[1]}</small>{/if}</dd></div>
			<div class="summary-devices"><dt>Devices online</dt><dd>{feed.current ? feed.status?.online : '—'}<small>/ {feed.status?.total ?? '—'}</small></dd></div>
		</dl>
		<p class="monitoring-live">{streaming ? 'Live · last 60 seconds' : 'Updates paused'}</p>
		<TrafficChart samples={feed.samples} live={streaming} />
	</section>
	<OverviewContext {server} {serverError} adguard={adguardError ? null : adguard} showDns={!!session.me?.adguard} totals={feed.current ? `${bytes(feed.status?.rx_bytes ?? 0)} / ${bytes(feed.status?.tx_bytes ?? 0)}` : '—'} />
</div>

<DeviceList {devices} current={feed.current} loading={!feed.status && !feed.error} unavailable={!!feed.error && !feed.status?.clients.length} selected={detailOpen ? selected ?? undefined : undefined} {now} rates={feed.rates} onselect={open} />
<DeviceDetails bind:open={detailOpen} {device} current={feed.current} {now} />
