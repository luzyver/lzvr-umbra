<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { createLiveStatus } from '#lib/live-status.svelte.ts';
	import DeviceList from '#lib/DeviceList.svelte';
	import DeviceDetails from '#lib/DeviceDetails.svelte';
	import Icon from '#lib/Icon.svelte';
	onMount(() => { if (new URL(location.href).searchParams.get('new') === '1') openNew(); });
	import { get, post, put, del, errMsg, type Client, type Defaults } from '#lib/api.ts';
	import { splitList } from '#lib/format.ts';
	import { session } from '#lib/session.svelte.ts';
	import { toast, warnAll } from '#lib/toast.svelte.ts';
	import Modal from '#lib/Modal.svelte';
	import Toggle from '#lib/Toggle.svelte';
	import ClientProfile from '#lib/ClientProfile.svelte';
	import ImportDevices from '#lib/ImportDevices.svelte';

	type Result = { client: Client; profile_changed: boolean; warnings: string[] };

	let clients = $state<Client[] | null>(null);
	let defaults = $state<Defaults | null>(null);
	let error = $state('');
	const feed = createLiveStatus();
	let selected = $state<number | null>(null);
	let detailOpen = $state(false);
	const rows = $derived((clients ?? []).map(c => {
		const telemetry = feed.status?.clients.find(s => s.id === c.id);
		return { ...telemetry, ...c, online: telemetry?.enabled === c.enabled ? telemetry.online : undefined };
	}));
	const selectedRow = $derived(rows.find(c => c.id === selected) ?? null);
	const selectedClient = $derived(clients?.find(c => c.id === selected) ?? null);
	const now = $derived(feed.status ? Date.parse(feed.status.time) : Date.now());
	let openedQuery = '';
	$effect(() => {
		const id = page.url.searchParams.get('device');
		if (id && clients?.some(c => c.id === Number(id)) && openedQuery !== id) {
			openedQuery = id; selected = Number(id); detailOpen = true;
		}
	});
	let importOpen = $state(false);

	async function load() {
		error = '';
		try {
			const [c, d] = await Promise.all([get<Client[]>('/clients'), get<Defaults>('/defaults')]);
			clients = c;
			defaults = d;
		} catch (e) {
			error = errMsg(e);
		}
	}
	load();

	function replace(c: Client) {
		if (!clients) return;
		const i = clients.findIndex((x) => x.id === c.id);
		if (i >= 0) clients[i] = c;
		else clients.push(c);
	}

	function afterSave(r: Result, msg: string) {
		warnAll(r.warnings);
		replace(r.client);
		if (r.profile_changed) toast(`${msg} The profile on the device needs to be downloaded again.`, 'warn', 7000);
		else toast(msg);
	}

	// ---- Add / edit form ----
	let formOpen = $state(false);
	let editing = $state<Client | null>(null);
	let busy = $state(false);
	let formError = $state('');
	let f = $state({ name: '', allowed_ips: '', dns: '', mtu: '', keepalive: '', adblock: true });

	function openNew() {
		editing = null;
		f = { name: '', allowed_ips: '', dns: '', mtu: '', keepalive: '', adblock: true };
		formError = '';
		formOpen = true;
	}

	function openEdit(c: Client) {
		editing = c;
		f = {
			name: c.name,
			allowed_ips: c.allowed_ips.join(', '),
			dns: c.dns.join(', '),
			mtu: c.mtu ? String(c.mtu) : '',
			keepalive: c.keepalive ? String(c.keepalive) : '',
			adblock: c.adblock
		};
		formError = '';
		formOpen = true;
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		formError = '';
		// Empty field = use the server default. The server stores values as given, so empty fields
		// are filled with the default (or omitted if defaults have not loaded) to avoid storing blanks.
		const body: Record<string, unknown> = { name: f.name.trim() };
		const list = (key: 'allowed_ips' | 'dns') => {
			const v = splitList(f[key]);
			if (v.length) body[key] = v;
			else if (defaults) body[key] = defaults[key];
		};
		const num = (key: 'mtu' | 'keepalive') => {
			const v = Number(f[key]);
			if (v) body[key] = v;
			else if (defaults) body[key] = defaults[key];
		};
		list('allowed_ips');
		list('dns');
		num('mtu');
		num('keepalive');
		if (session.me?.adguard) body.adblock = f.adblock;
		try {
			if (editing) {
				afterSave(await put<Result>(`/clients/${editing.id}`, body), 'Device saved.');
				formOpen = false;
			} else {
				const r = await post<Result>('/clients', body);
				warnAll(r.warnings);
				replace(r.client);
				formOpen = false;
				profile = r.client; // show the QR code right away
			}
		} catch (e) {
			formError = errMsg(e);
		} finally {
			busy = false;
		}
	}

	// ---- Quick toggles ----
	async function toggle(c: Client, field: 'enabled' | 'adblock', v: boolean) {
		try {
			const r = await put<Result>(`/clients/${c.id}`, { [field]: v });
			afterSave(
				r,
				field === 'enabled' ? (v ? `${c.name} enabled.` : `${c.name} disabled.`) : `Ad blocking ${v ? 'enabled' : 'disabled'} for ${c.name}.`
			);
		} catch (e) {
			toast(errMsg(e), 'error');
		}
	}

	async function regenerate(c: Client) {
		if (!confirm(`Regenerate keys for ${c.name}? The old profile will stop working; the device must import the new profile.`)) return;
		try {
			const r = await post<Result>(`/clients/${c.id}/regenerate-key`);
			replace(r.client); warnAll(r.warnings); detailOpen = false; profile = r.client;
		} catch (e) { toast(errMsg(e), 'error'); }
	}

	// ---- Delete ----
	let deleting = $state<Client | null>(null);
	let deleteOpen = $state(false);
	async function confirmDelete() {
		if (!deleting) return;
		const c = deleting;
		busy = true;
		try {
			const r = await del<{ warnings: string[] }>(`/clients/${c.id}`);
			warnAll(r?.warnings);
			clients = (clients ?? []).filter((x) => x.id !== c.id);
			toast(`${c.name} deleted.`);
			deleteOpen = false;
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	// ---- QR code ----
	let profile = $state<Client | null>(null);
	let profileOpen = $state(false);
	$effect(() => {
		if (profile) profileOpen = true;
	});
	$effect(() => {
		if (!profileOpen) profile = null;
	});
</script>

<svelte:head><title>Devices · Umbra</title></svelte:head>

<div class="page-heading">
	<h1>Devices</h1>
	<div class="heading-actions"><button class="btn-ghost" onclick={() => (importOpen = true)}><Icon name="down" size={16} />Import</button>
	<button class="btn-primary" onclick={openNew}><Icon name="plus" size={16} />Add device</button></div>
</div>

{#if error}
	<div class="inline-notice" role="alert">{error}<button class="text-action" onclick={load}>Try again</button></div>
{/if}
{#if feed.error}<div class="inline-notice" role="status">{feed.error}</div>{/if}
<DeviceList devices={rows} current={feed.current} loading={!clients && !error} unavailable={!clients && !!error} selected={detailOpen ? selected ?? undefined : undefined} {now} rates={feed.rates} heading="Device list" oncreate={openNew} onselect={id => { selected = id; detailOpen = true; }} />

<DeviceDetails bind:open={detailOpen} device={selectedRow} client={selectedClient} current={feed.current} {now}
	onedit={c => { detailOpen = false; openEdit(c); }}
	ondelete={c => { detailOpen = false; deleting = c; deleteOpen = true; }}
	onregenerate={regenerate}>
	{#snippet controls()}
		{#if selectedClient}
			<div><span>VPN access</span><Toggle checked={selectedClient.enabled} label="VPN access" onchange={v => selectedClient && toggle(selectedClient, 'enabled', v)} /></div>
			{#if session.me?.adguard}<div><span>Ad blocking</span><Toggle checked={selectedClient.adblock} label="Ad blocking" onchange={v => selectedClient && toggle(selectedClient, 'adblock', v)} /></div>{/if}
		{/if}
	{/snippet}
</DeviceDetails>

<Modal bind:open={formOpen} title={editing ? `Edit ${editing.name}` : 'Add device'}>
	<form id="client-form" class="space-y-4" onsubmit={save}>
		<div>
			<label class="label" for="c-name">Device name</label>
			<input id="c-name" class="input" bind:value={f.name} placeholder="Work laptop" required />
		</div>
		{#if session.me?.adguard}
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={f.adblock} /> Block ads & trackers (AdGuard)
			</label>
		{/if}
		<details>
			<summary class="cursor-pointer text-sm text-zinc-500">Advanced settings</summary>
			<div class="mt-3 space-y-3">
				<div>
					<label class="label" for="c-aips">Allowed IPs</label>
					<input id="c-aips" class="input font-mono" bind:value={f.allowed_ips} placeholder={defaults?.allowed_ips.join(', ')} />
					<p class="hint">Empty = default. 0.0.0.0/0 sends all internet traffic through the VPN.</p>
				</div>
				<div>
					<label class="label" for="c-dns">DNS</label>
					<input id="c-dns" class="input font-mono" bind:value={f.dns} placeholder={defaults?.dns.join(', ')} />
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div>
						<label class="label" for="c-mtu">MTU</label>
						<input id="c-mtu" class="input" type="number" min="0" max="1500" bind:value={f.mtu} placeholder={String(defaults?.mtu || 'auto')} />
					</div>
					<div>
						<label class="label" for="c-ka">Keepalive (seconds)</label>
						<input id="c-ka" class="input" type="number" min="0" max="3600" bind:value={f.keepalive} placeholder={String(defaults?.keepalive ?? '')} />
					</div>
				</div>
			</div>
		</details>
		{#if formError}<p class="text-sm text-red-600">{formError}</p>{/if}
	</form>
	{#snippet footer()}
		<button class="btn-ghost" onclick={() => (formOpen = false)}>Cancel</button>
		<button class="btn-primary" form="client-form" disabled={busy}>{busy ? 'Saving…' : 'Save'}</button>
	{/snippet}
</Modal>
<ImportDevices bind:open={importOpen} onimported={load} />

<Modal bind:open={deleteOpen} title="Delete device">
	<p class="text-sm">
		Delete <b>{deleting?.name}</b>? This device is disconnected immediately and its profile will stop working.
	</p>
	{#snippet footer()}
		<button class="btn-ghost" onclick={() => (deleteOpen = false)}>Cancel</button>
		<button class="btn-danger" disabled={busy} onclick={confirmDelete}>Delete</button>
	{/snippet}
</Modal>

<Modal drawer bind:open={profileOpen} title={profile ? `${profile.name} profile` : 'Profile'}>
	{#if profile}<ClientProfile client={profile} />{/if}
</Modal>
