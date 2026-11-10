<script lang="ts">
	import { page } from '$app/state';
	import { get, del, errMsg, type Activity, type ActivityPage, type Client } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import { bytes, ago, date, duration } from '#lib/format.ts';
	import NetActivity from '#lib/NetActivity.svelte';

	const kinds = [
		['', 'All'],
		['connect', 'Connected'],
		['disconnect', 'Disconnected'],
		['roam', 'IP change']
	] as const;
	const label: Record<Activity['kind'], string> = {
		connect: 'Connected',
		disconnect: 'Disconnected',
		roam: 'IP change'
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
	let tab = $state<'net' | 'sessions'>(page.url.searchParams.get('tab') === 'sessions' ? 'sessions' : 'net');
	const selected = $derived(clients.find((c) => String(c.id) === clientId));

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
		if (!confirm(who ? `Delete the activity log for ${who}?` : 'Delete the entire activity log?')) return;
		busy = true;
		try {
			const r = await del<{ deleted: number }>(`/activity${clientId ? `?client_id=${clientId}` : ''}`);
			toast(`${r.deleted} entries deleted.`);
			await load();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Activity · Umbra</title></svelte:head>

<div class="page-heading">
	<h1>Activity</h1>
	{#if tab === 'sessions'}
		<button class="btn-ghost ml-auto" disabled={busy} onclick={() => load()}>Reload</button>
		<button class="btn-danger" disabled={busy || !items?.length} onclick={clear}>Delete log</button>
	{/if}
</div>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<div class="filter-tabs" aria-label="View">
		<button class:selected={tab === 'net'} aria-pressed={tab === 'net'} onclick={() => (tab = 'net')}>Network activity</button>
		<button class:selected={tab === 'sessions'} aria-pressed={tab === 'sessions'} onclick={() => (tab = 'sessions')}>VPN sessions</button>
	</div>
	<select class="input ml-auto" aria-label="Device" bind:value={clientId} onchange={() => load()}>
		<option value="">{tab === 'net' ? 'Select device…' : 'All devices'}</option>
		{#each clients as c (c.id)}
			<option value={String(c.id)}>{c.name}</option>
		{/each}
	</select>
</div>

{#if tab === 'net'}
	{#if selected}
		<NetActivity client={selected} />
	{:else}
		<div class="card"><p class="hint">Select a device to see the sites and connections it is opening.</p></div>
	{/if}
{:else}
	<div class="card">
		<div class="mb-3 flex flex-wrap items-center gap-3">
			<div class="filter-tabs overflow-x-auto" aria-label="Event type">
				{#each kinds as [v, l] (v)}
					<button class:selected={kind === v} aria-pressed={kind === v} onclick={() => ((kind = v), load())}>{l}</button>
				{/each}
			</div>
		</div>

		{#if items}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead class="text-zinc-500">
						<tr>
							<th class="py-1 pr-3 font-medium">Time</th>
							<th class="py-1 pr-3 font-medium">Device</th>
							<th class="py-1 pr-3 font-medium">Event</th>
							<th class="py-1 pr-3 font-medium">Endpoint</th>
							<th class="py-1 pr-3 font-medium">Duration</th>
							<th class="py-1 font-medium">Session traffic</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-zinc-200 dark:divide-zinc-800">
						{#each items as a (a.id)}
							<tr>
								<td class="whitespace-nowrap py-1.5 pr-3 tabular-nums" title={ago(a.at)}>{date(a.at)}</td>
								<td class="py-1.5 pr-3">{a.client_name}</td>
								<td class="py-1.5 pr-3">
									<span class="rounded-sm px-2 py-0.5 {badge[a.kind]}">{label[a.kind]}</span>
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
									No activity yet. The panel checks WireGuard every ~15 seconds and records when a device
									connects, disconnects, or changes IP. Logs are kept for 90 days.
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
			{#if more}
				<button class="btn-ghost mt-3" disabled={busy} onclick={() => load(true)}>Load more</button>
			{/if}
		{:else}
			<p class="hint">Loading…</p>
		{/if}
		<p class="hint mt-3">
			WireGuard has no login/logout; a device counts as disconnected when there has been no handshake for 3 minutes,
			and the disconnect time is recorded as the last handshake. ↓ = received by the server from the device, ↑ = sent to the device.
		</p>
	</div>
{/if}
