import { onMount } from 'svelte';
import { get, type Status } from './api';
import { appendSample, sampleTraffic, type TrafficSample } from './traffic';

// One feed per mounted page; no sessions or timers survive navigation.
export function createLiveStatus() {
	const feed = $state({ status: null as Status | null, live: false, current: false, error: '', samples: [] as TrafficSample[] });
	onMount(() => {
		let active = true;
		let previous: Status | null = null;
		let stale: ReturnType<typeof setTimeout>;
		const disconnect = () => {
			feed.live = false;
			feed.current = false;
			feed.error = 'Pembaruan terputus. Mencoba menyambungkan kembali…';
			previous = null;
		};
		const update = (s: Status, streaming: boolean) => {
			if (!active) return;
			const time = Date.parse(s.time);
			if (!Number.isFinite(time) || (feed.status && time <= Date.parse(feed.status.time))) return;
			clearTimeout(stale);
			stale = setTimeout(disconnect, 15000);
			const sample = !s.error && previous ? sampleTraffic(previous, s) : null;
			feed.samples = sample ? appendSample(feed.samples, sample) : [];
			previous = s.error ? null : s;
			feed.status = s.error && !s.clients.length && feed.status
				? { ...s, clients: feed.status.clients, total: feed.status.total }
				: s;
			feed.current = !s.error && s.initialized;
			feed.live = streaming;
			feed.error = s.error ?? '';
		};
		get<Status>('/status').then(s => update(s, false)).catch(() => {
			if (active && !feed.status) feed.error = 'Status jaringan belum dapat dimuat. Mencoba menyambungkan kembali…';
		});
		const events = new EventSource('/api/events');
		events.addEventListener('status', e => {
			try { update(JSON.parse((e as MessageEvent).data), true); }
			catch { /* Leave the last valid snapshot until the freshness timer expires. */ }
		});
		events.onerror = disconnect;
		return () => { active = false; clearTimeout(stale); events.close(); };
	});
	return feed;
}
