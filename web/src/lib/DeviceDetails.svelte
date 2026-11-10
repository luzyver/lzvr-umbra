<script lang="ts">
	import type { Snippet } from 'svelte';
	import { get, errMsg, type Client } from './api';
	import { deviceState, stateLabels, type DeviceRow } from './devices';
	import { ago, bytes, date } from './format';
	import Modal from './Modal.svelte';
	import ClientProfile from './ClientProfile.svelte';
	import Icon from './Icon.svelte';
	let { open = $bindable(false), device, current, client, onedit, ondelete, onregenerate, controls, now = Date.now() }:
		{ open: boolean; device: DeviceRow | null; current: boolean; client?: Client | null; onedit?: (c: Client) => void; ondelete?: (c: Client) => void; onregenerate?: (c: Client) => void; controls?: Snippet; now?: number } = $props();
	let loaded = $state<Client | null>(null);
	let error = $state('');
	let attempt = $state(0);
	let tab = $state<'connection' | 'profile'>('connection');
	const full = $derived(client ?? loaded);
	const selectedId = $derived(device?.id);
	$effect(() => {
		const id = selectedId;
		const retry = attempt;
		if (!open || !id) return;
		tab = 'connection';
		loaded = null;
		error = '';
		if (client) return;
		let active = true;
		get<Client>(`/clients/${id}`).then(c => { if (active) loaded = c; }).catch(e => { if (active) error = errMsg(e); });
		return () => { active = false; };
	});
</script>

<Modal bind:open title={device?.name ?? 'Device details'} drawer>
	{#if device}
		{@const state = deviceState(device, current)}
		<div class="device-detail-summary"><span class="table-mono">{device.address}</span><span class="device-status {state}"><i class="status-led"></i>{stateLabels[state]}</span></div>
		<div class="detail-tabs" aria-label="Device detail sections">
			<button class:selected={tab === 'connection'} aria-pressed={tab === 'connection'} onclick={() => tab = 'connection'}>Connection</button>
			<button class:selected={tab === 'profile'} aria-pressed={tab === 'profile'} onclick={() => tab = 'profile'}>Profile & QR</button>
		</div>
		{#if tab === 'connection'}
			<div class="detail-transfer"><div><span><Icon name="down" size={14} />Data in</span><strong>{current && device.rx_bytes !== undefined ? bytes(device.rx_bytes) : '—'}</strong></div><div><span><Icon name="up" size={14} />Data out</span><strong>{current && device.tx_bytes !== undefined ? bytes(device.tx_bytes) : '—'}</strong></div></div>
			<dl class="detail-facts">
				<div><dt>Last handshake</dt><dd>{current && device.online !== undefined ? ago(device.last_handshake, now) : 'Unknown'}</dd></div>
				<div><dt>VPN access</dt><dd>{device.enabled ? 'Allowed' : 'Disabled'}</dd></div>
				{#if full}
					<div><dt>DNS address</dt><dd class="table-mono">{full.dns.join(', ') || 'Server default'}</dd></div>
					<div><dt>Allowed IPs</dt><dd class="table-mono">{full.allowed_ips.join(', ')}</dd></div>
					<div><dt>Created</dt><dd>{date(full.created_at)}</dd></div>
				{/if}
			</dl>
			{#if controls}<div class="detail-controls">{@render controls()}</div>{/if}
			<a class="detail-activity text-action" href="/activity?client={device.id}">Network activity <Icon name="arrow" size={16} /></a>
		{:else if full}
			{#if full.has_private_key}<ClientProfile client={full} showActivity={false} />
			{:else}<div class="profile-unavailable"><p>Private key not available.</p><p class="hint">Imported devices can still connect. Regenerate keys to get a new profile and QR code.</p>{#if onregenerate}<button class="btn-ghost mt-4" onclick={() => full && onregenerate?.(full)}>Regenerate keys</button>{:else}<a class="btn-ghost mt-4" href="/clients?device={device.id}">Manage device</a>{/if}</div>{/if}
		{/if}
		{#if error}<div class="inline-notice" role="alert">{error}<button class="text-action" onclick={() => attempt++}>Try again</button></div>{:else if !full}<p class="hint mt-4" role="status">Loading device profile…</p>{/if}
	{/if}
	{#snippet footer()}
		{#if full && onedit}<button class="btn-ghost" onclick={() => full && onedit?.(full)}>Edit device</button>{:else if device}<a class="btn-ghost" href="/clients?device={device.id}">Manage device</a>{/if}
		{#if full && ondelete}<button class="btn-ghost text-red-600" onclick={() => full && ondelete?.(full)}>Delete</button>{/if}
	{/snippet}
</Modal>
