<script lang="ts">
	import { onMount } from 'svelte';
	import { deviceState, stateLabels, type DeviceRow } from './devices';
	import { portPanel } from './network';
	import { BUSY_RATE } from './traffic';
	let { address, name, devices, known, current, rates = {}, selected, onselect }:
		{ address: string; name: string; devices: DeviceRow[]; known: boolean; current: boolean; rates?: Record<number, number>; selected?: number; onselect: (id: number) => void } = $props();
	const panel = $derived(portPanel(address, devices));
	// The lights switch on in sequence once when the panel appears; after that only changed lights blink.
	let sweep = $state(true);
	onMount(() => { const t = setTimeout(() => sweep = false, 1600); return () => clearTimeout(t); });
</script>

{#if panel}
	<section class="ports" aria-label="VPN addresses">
		<div class="ports-caption">
			<span><strong>Addresses {panel.network}</strong> · {name}</span>
			<!-- Without the device list an empty port may not really be empty: do not state counts or the next address. -->
			<span role="status">{#if known}{panel.used} used · {panel.free} free{panel.next ? ` · next ${panel.next}` : ' · subnet full'}{:else}Address usage unknown{/if}</span>
		</div>
		<div class="ports-row" class:sweep>
			{#each panel.ports as port, i (port.address)}
				{#if port.device}
					{@const d = port.device}
					{@const state = deviceState(d, current)}
					{@const busy = state === 'online' && (rates[d.id] ?? 0) >= BUSY_RATE}
					{@const label = busy ? 'Online, carrying traffic' : stateLabels[state]}
					<button class="port {state}" class:busy class:selected={selected === d.id} style="--i:{i}" title="{d.name} · {port.address} · {label}" aria-label="{d.name}, {port.address}, {label}" onclick={() => onselect(d.id)}>
						{#key busy ? 'busy' : state}<i></i>{/key}{port.label}
					</button>
				{:else}
					<span class="port" style="--i:{i}" title={known ? `${port.address} free` : port.address} aria-hidden="true"><i></i>{port.label}</span>
				{/if}
			{/each}
		</div>
		{#if known && panel.hidden}<p class="ports-more">{panel.hidden} more devices are at higher addresses; see the table below.</p>{/if}
	</section>
{/if}
