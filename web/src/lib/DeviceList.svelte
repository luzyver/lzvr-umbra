<script lang="ts">
	import Icon from './Icon.svelte';
	import { ago, bytes } from './format';
	import { BUSY_RATE } from './traffic';
	import { deviceState, selectDevices, stateLabels, type DeviceFilter, type DeviceRow } from './devices';
	let { devices, current, loading = false, unavailable = false, selected, onselect, oncreate, heading = 'Devices', now = Date.now(), rates = {} }:
		{ devices: DeviceRow[]; current: boolean; loading?: boolean; unavailable?: boolean; selected?: number; onselect: (id: number) => void; oncreate?: () => void; heading?: string; now?: number; rates?: Record<number, number> } = $props();
	let query = $state('');
	let filter = $state<DeviceFilter>('all');
	const filters: { value: DeviceFilter; label: string }[] = [
		{ value: 'all', label: 'All' }, { value: 'online', label: 'Online' },
		{ value: 'offline', label: 'Offline' }, { value: 'disabled', label: 'Disabled' }
	];
	const shown = $derived(selectDevices(devices, query, filter, current));
	function selectRow(event: MouseEvent, id: number) {
		// Buttons own their activation, including clicks from Enter/Space.
		if (event.target instanceof Element && event.target.closest('button')) return;
		onselect(id);
	}
</script>

<section class="device-section" aria-label={heading}>
	<div class="device-toolbar">
		<h2 class="section-title">{heading}</h2>
		<div class="device-filters" aria-label="Filter by status">
			{#each filters as tab}
				<button class:selected={filter === tab.value} aria-pressed={filter === tab.value} onclick={() => filter = tab.value}>{tab.label}<span class="filter-count">{loading || unavailable || (!current && (tab.value === 'online' || tab.value === 'offline')) ? '—' : selectDevices(devices, '', tab.value, current).length}</span></button>
			{/each}
		</div>
		<label class="device-search"><Icon name="search" size={16} /><input class="input" aria-label="Search devices" placeholder="Search name or IP…" bind:value={query} /></label>
	</div>
	{#if loading}
		<div class="device-empty" role="status">Loading devices…</div>
	{:else if unavailable}
		<div class="device-empty" role="status">The device list could not be loaded.</div>
	{:else if !shown.length}
		<div class="device-empty">
			<p>{devices.length ? 'No matching devices.' : 'No devices yet.'}</p>
			{#if query || filter !== 'all'}<button class="text-action" onclick={() => { query = ''; filter = 'all'; }}>Clear filters</button>
			{:else if oncreate}<button class="text-action" onclick={oncreate}>Add your first device <Icon name="arrow" size={14} /></button>
			{:else}<a class="text-action" href="/clients?new=1">Add your first device <Icon name="arrow" size={14} /></a>{/if}
		</div>
	{:else}
		<table class="device-table">
			<thead><tr><th scope="col">Device</th><th scope="col">IP</th><th scope="col">Status</th><th scope="col" class="num">In</th><th scope="col" class="num">Out</th><th scope="col" class="num">Last handshake</th></tr></thead>
			<tbody>
				{#each shown as device (device.id)}
					{@const state = deviceState(device, current)}
					{@const busy = state === 'online' && (rates[device.id] ?? 0) >= BUSY_RATE}
					<!-- Preserve table semantics; the name button provides keyboard access. -->
					<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_noninteractive_element_interactions -->
					<tr class="device-row" class:selected={selected === device.id} onclick={event => selectRow(event, device.id)}>
						<td class="device-identity"><button class="device-open" aria-label="Details for {device.name}, {device.address}" onclick={() => onselect(device.id)}><span class="tape {state}">{device.name}</span></button></td>
						<td class="device-address" class:long={device.address.length > 18}>{device.address.replace(/\/32$/, '')}</td>
						<td class="device-state"><span class="device-status {state}" class:busy><i class="status-led"></i>{busy ? 'Online, carrying traffic' : stateLabels[state]}</span></td>
						<td class="device-bytes num" title="Data received by the server"><span class="mobile-field">In</span>{current && device.rx_bytes !== undefined ? bytes(device.rx_bytes) : '—'}</td>
						<td class="device-bytes num" title="Data sent by the server"><span class="mobile-field">Out</span>{current && device.tx_bytes !== undefined ? bytes(device.tx_bytes) : '—'}</td>
						<td class="device-handshake num"><span class="mobile-field">Handshake</span><span title={device.last_handshake ? new Date(device.last_handshake).toLocaleString('en-GB') : undefined}>{current && device.online !== undefined ? ago(device.last_handshake, now) : '—'}</span></td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
	<div class="device-table-foot"><span>{shown.length} of {devices.length} devices</span><span>Traffic is counted on the server side</span></div>
</section>

<style>
	.device-row { cursor: pointer; }
</style>
