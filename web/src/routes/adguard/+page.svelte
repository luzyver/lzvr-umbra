<script lang="ts">
	import { onDestroy } from 'svelte';
	import {
		get,
		post,
		errMsg,
		ApiError,
		type AdGuardStatus,
		type AdGuardJob,
		type AdGuardSync,
		type Me
	} from '#lib/api.ts';
	import { toast, warnAll } from '#lib/toast.svelte.ts';
	import { session } from '#lib/session.svelte.ts';
	import Setup from '#lib/adguard/Setup.svelte';
	import Overview from '#lib/adguard/Overview.svelte';
	import Filters from '#lib/adguard/Filters.svelte';
	import Dns from '#lib/adguard/Dns.svelte';
	import Services from '#lib/adguard/Services.svelte';
	import Clients from '#lib/adguard/Clients.svelte';
	import QueryLog from '#lib/adguard/QueryLog.svelte';
	import Connection from '#lib/adguard/Connection.svelte';

	const tabs = [
		['overview', 'Overview'],
		['filters', 'Filters'],
		['dns', 'DNS'],
		['services', 'Services'],
		['clients', 'Clients'],
		['log', 'Log'],
		['conn', 'Connection']
	] as const;
	type Tab = (typeof tabs)[number][0];

	let st = $state<AdGuardStatus | null>(null);
	let job = $state<AdGuardJob | null>(null);
	let error = $state('');
	let busy = $state(false);
	let tab = $state<Tab>('overview');
	let timer: ReturnType<typeof setInterval> | undefined;

	async function load() {
		error = '';
		try {
			st = await get<AdGuardStatus>('/adguard/status');
		} catch (e) {
			if (e instanceof ApiError && e.status === 503) st = { configured: false };
			else error = errMsg(e);
		}
		await loadJob();
	}

	async function loadJob() {
		const prev = job?.state;
		try {
			job = await get<AdGuardJob>('/adguard/install');
		} catch {
			return;
		}
		if (job.state === 'running') {
			timer ??= setInterval(loadJob, 2000);
			return;
		}
		clearInterval(timer);
		timer = undefined;
		if (prev === 'running') {
			// The install/uninstall just finished: show the result and reload everything.
			toast(job.message, job.state === 'error' ? 'error' : 'ok');
			await refreshMe();
			tab = 'overview';
			await load();
		}
	}

	async function refreshMe() {
		session.me = await get<Me>('/auth/me').catch(() => session.me);
	}

	async function changed() {
		await refreshMe();
		await load();
	}

	onDestroy(() => clearInterval(timer));
	load();

	async function sync() {
		busy = true;
		try {
			const r = await post<AdGuardSync>('/adguard/sync');
			warnAll(r.warnings);
			toast(`Sync complete: ${r.synced} devices updated, ${r.removed} removed.`);
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	const running = $derived(job?.state === 'running');
</script>

<svelte:head><title>AdGuard · Umbra</title></svelte:head>

<div class="page-heading">
	<h1>AdGuard</h1>
	{#if st?.configured && st.reachable && !running}
		<button class="btn-ghost ml-auto" disabled={busy} onclick={load}>Reload</button>
		<button class="btn-primary" disabled={busy} onclick={sync}>Sync devices</button>
	{/if}
</div>

{#if error}
	<div class="card text-sm text-red-600">{error}</div>
{:else if !st || !job}
	<p class="text-sm text-zinc-500">Loading…</p>
{:else if running && job.action === 'uninstall'}
	<div class="card space-y-2 text-sm">
		<p class="font-medium">Uninstalling AdGuard Home…</p>
		<p class="hint">{job.message || 'Please wait.'}</p>
	</div>
{:else if !st.configured || running}
	<Setup {job} ondone={loadJob} />
{:else}
	{#if !st.reachable}
		<div class="card mb-4 space-y-2 text-sm">
			<p class="font-medium text-red-600">AdGuard Home could not be reached.</p>
			{#if st.error}<p class="font-mono text-xs text-zinc-500">{st.error}</p>{/if}
			<p class="hint">Check that the AdGuard service is running, or fix its address or password below.</p>
		</div>
		<Connection onchange={changed} />
	{:else if st.status}
		<div class="filter-tabs mb-4 overflow-x-auto">
			{#each tabs as [id, label] (id)}
				<button class:selected={tab === id} aria-pressed={tab === id} onclick={() => (tab = id)}>{label}</button>
			{/each}
		</div>

		{#if tab === 'overview'}
			<Overview st={st.status} onchange={load} />
		{:else if tab === 'filters'}
			<Filters />
		{:else if tab === 'dns'}
			<Dns />
		{:else if tab === 'services'}
			<Services />
		{:else if tab === 'clients'}
			<Clients />
		{:else if tab === 'log'}
			<QueryLog />
		{:else}
			<Connection onchange={changed} />
		{/if}
	{/if}
{/if}
