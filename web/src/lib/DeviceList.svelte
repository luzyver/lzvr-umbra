<script lang="ts">
	import Icon from './Icon.svelte';
	import { ago, bytes } from './format';
	import { deviceState, selectDevices, stateLabels, type DeviceFilter, type DeviceRow } from './devices';
	let { devices, current, loading = false, unavailable = false, selected, onselect, oncreate, heading = 'Perangkat', now = Date.now() }:
		{ devices: DeviceRow[]; current: boolean; loading?: boolean; unavailable?: boolean; selected?: number; onselect: (id: number) => void; oncreate?: () => void; heading?: string; now?: number } = $props();
	let query = $state('');
	let filter = $state<DeviceFilter>('all');
	const filters: { value: DeviceFilter; label: string }[] = [
		{ value: 'all', label: 'Semua' }, { value: 'online', label: 'Online' },
		{ value: 'offline', label: 'Offline' }, { value: 'disabled', label: 'Nonaktif' }
	];
	const shown = $derived(selectDevices(devices, query, filter, current));
	function selectRow(event: MouseEvent, id: number) {
		// Buttons own their activation, including clicks from Enter/Space.
		if (event.target instanceof Element && event.target.closest('button')) return;
		onselect(id);
	}
</script>

<section class="device-section" aria-label={heading}>
	<div class="device-section-heading">
		<h2 class="section-title">{heading} <span class="count-label">{loading ? '—' : devices.length}</span></h2>
		<span class="device-caption">Pilih perangkat untuk melihat detail</span>
	</div>
	<div class="device-surface">
		<div class="device-toolbar">
			<div class="device-filters" aria-label="Filter status">
				{#each filters as tab}
					<button class:selected={filter === tab.value} aria-pressed={filter === tab.value} onclick={() => filter = tab.value}>{tab.label}<span class="filter-count">{loading || unavailable || (!current && (tab.value === 'online' || tab.value === 'offline')) ? '—' : selectDevices(devices, '', tab.value, current).length}</span></button>
				{/each}
			</div>
			<label class="device-search"><Icon name="search" size={16} /><input class="input" aria-label="Cari perangkat" placeholder="Cari nama atau IP…" bind:value={query} /></label>
		</div>
		{#if loading}
			<div class="device-empty" role="status">Memuat perangkat…</div>
		{:else if unavailable}
			<div class="device-empty" role="status">Daftar perangkat belum dapat dimuat.</div>
		{:else if !shown.length}
			<div class="device-empty">
				<p>{devices.length ? 'Tidak ada perangkat yang sesuai.' : 'Belum ada perangkat.'}</p>
				{#if query || filter !== 'all'}<button class="text-action" onclick={() => { query = ''; filter = 'all'; }}>Hapus filter</button>
				{:else if oncreate}<button class="text-action" onclick={oncreate}>Tambah perangkat pertama <Icon name="arrow" size={14} /></button>
				{:else}<a class="text-action" href="/clients?new=1">Tambah perangkat pertama <Icon name="arrow" size={14} /></a>{/if}
			</div>
		{:else}
			<table class="device-table">
				<thead><tr><th scope="col">Perangkat</th><th scope="col">Status</th><th scope="col" class="traffic-column">Masuk / keluar</th><th scope="col">Handshake terakhir</th><th scope="col"><span class="sr-only">Detail</span></th></tr></thead>
				<tbody>
					{#each shown as device (device.id)}
						{@const state = deviceState(device, current)}
						<!-- Preserve table semantics; the name and arrow buttons provide keyboard access. -->
						<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
						<tr class="device-row" class:selected={selected === device.id} onclick={event => selectRow(event, device.id)}>
							<td class="device-identity"><button class="device-open" aria-label="Detail {device.name}, {device.address}" onclick={() => onselect(device.id)}><span class="device-name">{device.name}</span><span class="table-mono">{device.address}</span></button></td>
							<td class="device-state"><span class="device-status" class:online={state === 'online'} class:disabled={state === 'disabled'}><i class="status-dot"></i>{stateLabels[state]}</span></td>
							<td class="device-traffic"><span title="Data masuk ke server"><span class="sr-only">Masuk ke server: </span><Icon name="down" size={13} />{current && device.rx_bytes !== undefined ? bytes(device.rx_bytes) : '—'}</span><span title="Data keluar dari server"><span class="sr-only">Keluar dari server: </span><Icon name="up" size={13} />{current && device.tx_bytes !== undefined ? bytes(device.tx_bytes) : '—'}</span></td>
							<td class="device-handshake"><span class="mobile-field">Handshake</span><span title={device.last_handshake ? new Date(device.last_handshake).toLocaleString('id-ID') : undefined}>{current && device.online !== undefined ? ago(device.last_handshake, now) : '—'}</span></td>
							<td class="device-more"><button class="icon-btn" aria-label="Buka {device.name}" onclick={() => onselect(device.id)}><Icon name="arrow" size={16} /></button></td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
		<div class="device-table-foot"><span>{shown.length} dari {devices.length} perangkat</span><span>Trafik dari sisi server</span></div>
	</div>
</section>

<style>
	.device-row { cursor: pointer; }
</style>
