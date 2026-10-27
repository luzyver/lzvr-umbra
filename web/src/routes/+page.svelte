<script lang="ts">
	import { onDestroy } from 'svelte';
	import { get, type Status } from '#lib/api.ts';
	import { bytes, rate, ago } from '#lib/format.ts';

	let status = $state<Status | null>(null);
	let live = $state(false);
	// Kecepatan per klien dihitung dari selisih dua event berturut-turut.
	let rates = $state<Record<number, { rx: number; tx: number }>>({});
	let prev: { t: number; c: Record<number, { rx: number; tx: number }> } | null = null;

	function update(s: Status) {
		const t = new Date(s.time).getTime();
		const cur: Record<number, { rx: number; tx: number }> = {};
		const next: Record<number, { rx: number; tx: number }> = {};
		for (const c of s.clients) {
			cur[c.id] = { rx: c.rx_bytes, tx: c.tx_bytes };
			const p = prev?.c[c.id];
			const dt = prev ? (t - prev.t) / 1000 : 0;
			next[c.id] =
				p && dt > 0
					? { rx: Math.max(0, (c.rx_bytes - p.rx) / dt), tx: Math.max(0, (c.tx_bytes - p.tx) / dt) }
					: { rx: 0, tx: 0 };
		}
		prev = { t, c: cur };
		rates = next;
		status = s;
	}

	get<Status>('/status').then(update).catch(() => {});

	const es = new EventSource('/api/events');
	es.addEventListener('status', (e) => {
		live = true;
		try {
			update(JSON.parse((e as MessageEvent).data));
		} catch {
			/* abaikan event rusak */
		}
	});
	es.onerror = () => (live = false);
	onDestroy(() => es.close());

	// Diurutkan: online dulu, lalu yang terakhir handshake paling baru.
	const clients = $derived(
		[...(status?.clients ?? [])].sort(
			(a, b) =>
				Number(b.online) - Number(a.online) ||
				(b.last_handshake ? Date.parse(b.last_handshake) : 0) - (a.last_handshake ? Date.parse(a.last_handshake) : 0)
		)
	);
	const totalRate = $derived(
		Object.values(rates).reduce((s, r) => ({ rx: s.rx + r.rx, tx: s.tx + r.tx }), { rx: 0, tx: 0 })
	);
</script>

<svelte:head><title>Dasbor · Umbra</title></svelte:head>

<div class="mb-4 flex items-center justify-between">
	<h1 class="text-xl font-semibold">Dasbor</h1>
	<span class="flex items-center gap-2 text-xs text-zinc-500">
		<span class="h-2 w-2 rounded-full {live ? 'bg-teal-500' : 'bg-zinc-400'}"></span>
		{live ? 'langsung' : 'menyambung…'}
	</span>
</div>

{#if status?.error}
	<div class="card mb-4 border-red-300 text-sm text-red-700 dark:border-red-800 dark:text-red-300">
		{status.error}
	</div>
{/if}

<div class="mb-6 grid gap-4 sm:grid-cols-3">
	<div class="card">
		<p class="hint">Klien online</p>
		<p class="mt-1 text-2xl font-semibold tabular-nums">
			{status?.online ?? '–'}<span class="text-base font-normal text-zinc-500"> / {status?.total ?? '–'}</span>
		</p>
	</div>
	<div class="card">
		<p class="hint">Diterima server</p>
		<p class="mt-1 text-2xl font-semibold tabular-nums">{status ? bytes(status.rx_bytes) : '–'}</p>
		<p class="hint tabular-nums">{rate(totalRate.rx)}</p>
	</div>
	<div class="card">
		<p class="hint">Dikirim server</p>
		<p class="mt-1 text-2xl font-semibold tabular-nums">{status ? bytes(status.tx_bytes) : '–'}</p>
		<p class="hint tabular-nums">{rate(totalRate.tx)}</p>
	</div>
</div>

<div class="card overflow-x-auto p-0">
	{#if !status}
		<p class="p-6 text-sm text-zinc-500">Memuat…</p>
	{:else if clients.length === 0}
		<p class="p-6 text-sm text-zinc-500">
			Belum ada klien. <a class="text-teal-700 underline dark:text-teal-400" href="/clients">Tambah klien</a>
		</p>
	{:else}
		<table class="w-full text-sm">
			<thead class="text-left text-xs text-zinc-500">
				<tr class="border-b border-zinc-200 dark:border-zinc-800">
					<th class="px-4 py-2 font-medium">Klien</th>
					<th class="px-4 py-2 font-medium">Status</th>
					<th class="px-4 py-2 font-medium">Handshake terakhir</th>
					<th class="px-4 py-2 font-medium">Dari</th>
					<th class="px-4 py-2 text-right font-medium">Unduh / unggah</th>
				</tr>
			</thead>
			<tbody>
				{#each clients as c (c.id)}
					<tr class="border-b border-zinc-100 last:border-0 dark:border-zinc-800/60">
						<td class="px-4 py-2">
							<div class="font-medium">{c.name}</div>
							<div class="font-mono text-xs text-zinc-500">{c.address}</div>
						</td>
						<td class="px-4 py-2">
							{#if !c.enabled}
								<span class="badge bg-zinc-200 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">nonaktif</span>
							{:else if c.online}
								<span class="badge bg-teal-100 text-teal-800 dark:bg-teal-950 dark:text-teal-200">online</span>
							{:else}
								<span class="badge bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-400">offline</span>
							{/if}
						</td>
						<td class="px-4 py-2 text-zinc-600 dark:text-zinc-400">{ago(c.last_handshake)}</td>
						<td class="px-4 py-2 font-mono text-xs text-zinc-500">{c.endpoint || '–'}</td>
						<td class="px-4 py-2 text-right tabular-nums">
							<!-- dari sisi server: rx = data dari klien (unggah klien) -->
							<div>↓ {bytes(c.tx_bytes)} · ↑ {bytes(c.rx_bytes)}</div>
							{#if c.online}
								<div class="text-xs text-zinc-500">↓ {rate(rates[c.id]?.tx ?? 0)} · ↑ {rate(rates[c.id]?.rx ?? 0)}</div>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</div>
