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

<Modal bind:open title={device?.name ?? 'Detail perangkat'} drawer>
	{#if device}
		{@const state = deviceState(device, current)}
		<div class="device-detail-summary"><span class="table-mono">{device.address}</span><span class="device-status {state}"><i class="status-led"></i>{stateLabels[state]}</span></div>
		<div class="detail-tabs" aria-label="Bagian detail perangkat">
			<button class:selected={tab === 'connection'} aria-pressed={tab === 'connection'} onclick={() => tab = 'connection'}>Koneksi</button>
			<button class:selected={tab === 'profile'} aria-pressed={tab === 'profile'} onclick={() => tab = 'profile'}>Profil & QR</button>
		</div>
		{#if tab === 'connection'}
			<div class="detail-transfer"><div><span><Icon name="down" size={14} />Data masuk</span><strong>{current && device.rx_bytes !== undefined ? bytes(device.rx_bytes) : '—'}</strong></div><div><span><Icon name="up" size={14} />Data keluar</span><strong>{current && device.tx_bytes !== undefined ? bytes(device.tx_bytes) : '—'}</strong></div></div>
			<dl class="detail-facts">
				<div><dt>Handshake terakhir</dt><dd>{current && device.online !== undefined ? ago(device.last_handshake, now) : 'Belum diketahui'}</dd></div>
				<div><dt>Akses VPN</dt><dd>{device.enabled ? 'Diizinkan' : 'Nonaktif'}</dd></div>
				{#if full}
					<div><dt>Alamat DNS</dt><dd class="table-mono">{full.dns.join(', ') || 'Bawaan server'}</dd></div>
					<div><dt>Allowed IPs</dt><dd class="table-mono">{full.allowed_ips.join(', ')}</dd></div>
					<div><dt>Dibuat</dt><dd>{date(full.created_at)}</dd></div>
				{/if}
			</dl>
			{#if controls}<div class="detail-controls">{@render controls()}</div>{/if}
			<a class="detail-activity text-action" href="/activity?client={device.id}">Aktivitas jaringan <Icon name="arrow" size={16} /></a>
		{:else if full}
			{#if full.has_private_key}<ClientProfile client={full} showActivity={false} />
			{:else}<div class="profile-unavailable"><p>Kunci privat tidak tersedia.</p><p class="hint">Perangkat hasil impor tetap dapat terhubung. Buat ulang kunci untuk mendapatkan profil dan QR baru.</p>{#if onregenerate}<button class="btn-ghost mt-4" onclick={() => full && onregenerate?.(full)}>Buat ulang kunci</button>{:else}<a class="btn-ghost mt-4" href="/clients?device={device.id}">Kelola perangkat</a>{/if}</div>{/if}
		{/if}
		{#if error}<div class="inline-notice" role="alert">{error}<button class="text-action" onclick={() => attempt++}>Coba lagi</button></div>{:else if !full}<p class="hint mt-4" role="status">Memuat profil perangkat…</p>{/if}
	{/if}
	{#snippet footer()}
		{#if full && onedit}<button class="btn-ghost" onclick={() => full && onedit?.(full)}>Ubah perangkat</button>{:else if device}<a class="btn-ghost" href="/clients?device={device.id}">Kelola perangkat</a>{/if}
		{#if full && ondelete}<button class="btn-ghost text-red-600" onclick={() => full && ondelete?.(full)}>Hapus</button>{/if}
	{/snippet}
</Modal>
