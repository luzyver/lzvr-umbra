<script lang="ts">
	import { page } from '$app/state';
	import { get, del, errMsg, type Activity, type ActivityPage, type Client } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { bytes, ago, date, duration } from '#lib/format.ts';

	const kinds = [
		['', 'Semua'],
		['connect', 'Terhubung'],
		['disconnect', 'Terputus'],
		['roam', 'Ganti IP']
	] as const;
	const label: Record<Activity['kind'], string> = {
		connect: 'Terhubung',
		disconnect: 'Terputus',
		roam: 'Ganti IP'
	};
	const badge: Record<Activity['kind'], string> = {
		connect: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300',
		disconnect: 'bg-zinc-200 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300',
		roam: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300'
	};

	let clients = $state<Client[]>([]);
	let clientId = $state(page.url.searchParams.get('client') ?? '');
	let kind = $state('');
	let items = $state<Activity[] | null>(null);
	let more = $state(false);
	let busy = $state(false);

	async function load(append = false) {
		busy = true;
		const params = new URLSearchParams({ limit: '50' });
		if (clientId) params.set('client_id', clientId);
		if (kind) params.set('kind', kind);
		if (append && items?.length) params.set('before', String(items[items.length - 1].id));
		try {
			const r = await get<ActivityPage>(`/activity?${params}`);
			items = append && items ? [...items, ...r.items] : r.items;
			more = r.more;
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
	load();
	get<Client[]>('/clients')
		.then((r) => (clients = r))
		.catch(() => {});

	async function clear() {
		const who = clients.find((c) => String(c.id) === clientId)?.name;
		if (!confirm(who ? `Hapus log aktivitas ${who}?` : 'Hapus seluruh log aktivitas?')) return;
		busy = true;
		try {
			const r = await del<{ deleted: number }>(`/activity${clientId ? `?client_id=${clientId}` : ''}`);
			toast(`${r.deleted} entri dihapus.`);
			await load();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Aktivitas · Umbra</title></svelte:head>

<div class="page-heading">
	<div><div class="overline">CONNECTION LOG</div><h1>Aktivitas</h1><p>Kapan perangkat terhubung, terputus, dan dari mana.</p></div>
	<button class="btn-ghost ml-auto" disabled={busy} onclick={() => load()}>Muat ulang</button>
	<button class="btn-danger" disabled={busy || !items?.length} onclick={clear}>Hapus log</button>
</div>

<div class="card">
	<div class="mb-3 flex flex-wrap items-center gap-3">
		<div class="filter-tabs overflow-x-auto" aria-label="Jenis kejadian">
			{#each kinds as [v, l] (v)}
				<button class:selected={kind === v} aria-pressed={kind === v} onclick={() => ((kind = v), load())}>{l}</button>
			{/each}
		</div>
		<select class="input ml-auto" aria-label="Perangkat" bind:value={clientId} onchange={() => load()}>
			<option value="">Semua perangkat</option>
			{#each clients as c (c.id)}
				<option value={String(c.id)}>{c.name}</option>
			{/each}
		</select>
	</div>

	{#if items}
		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead class="text-zinc-500">
					<tr>
						<th class="py-1 pr-3 font-medium">Waktu</th>
						<th class="py-1 pr-3 font-medium">Perangkat</th>
						<th class="py-1 pr-3 font-medium">Kejadian</th>
						<th class="py-1 pr-3 font-medium">Endpoint</th>
						<th class="py-1 pr-3 font-medium">Durasi</th>
						<th class="py-1 font-medium">Trafik sesi</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-zinc-200 dark:divide-zinc-800">
					{#each items as a (a.id)}
						<tr>
							<td class="whitespace-nowrap py-1.5 pr-3 tabular-nums" title={ago(a.at)}>{date(a.at)}</td>
							<td class="py-1.5 pr-3">{a.client_name}</td>
							<td class="py-1.5 pr-3">
								<span class="rounded-full px-2 py-0.5 {badge[a.kind]}">{label[a.kind]}</span>
								{#if a.detail && a.kind !== 'roam'}<span class="ml-1 text-zinc-500">{a.detail}</span>{/if}
							</td>
							<td class="whitespace-nowrap py-1.5 pr-3 font-mono">
								{#if a.kind === 'roam' && a.detail}<span class="text-zinc-500">{a.detail} →</span>{/if}
								{a.endpoint ?? '–'}
							</td>
							<td class="whitespace-nowrap py-1.5 pr-3 tabular-nums">{a.kind === 'disconnect' ? duration(a.duration) : '–'}</td>
							<td class="whitespace-nowrap py-1.5 tabular-nums">
								{#if a.kind === 'disconnect'}↓ {bytes(a.rx_bytes)} · ↑ {bytes(a.tx_bytes)}{:else}–{/if}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="6" class="hint py-3">
								Belum ada aktivitas. Panel mengecek WireGuard tiap ±15 detik dan mencatat saat perangkat
								terhubung, terputus, atau berganti IP. Log disimpan 90 hari.
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
		{#if more}
			<button class="btn-ghost mt-3" disabled={busy} onclick={() => load(true)}>Muat lebih banyak</button>
		{/if}
	{:else}
		<p class="hint">Memuat…</p>
	{/if}
	<p class="hint mt-3">
		WireGuard tidak punya login/logout; perangkat dianggap terputus bila tidak ada handshake selama 3 menit,
		dan waktu putusnya dicatat sebagai handshake terakhir. ↓ = diterima server dari perangkat, ↑ = dikirim ke perangkat.
	</p>
</div>
