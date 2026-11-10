<script lang="ts">
	import { onMount } from 'svelte';
	import { deviceState, stateLabels, type DeviceRow } from './devices';
	import { portPanel } from './network';
	import { BUSY_RATE } from './traffic';
	let { address, name, devices, known, current, rates = {}, selected, onselect }:
		{ address: string; name: string; devices: DeviceRow[]; known: boolean; current: boolean; rates?: Record<number, number>; selected?: number; onselect: (id: number) => void } = $props();
	const panel = $derived(portPanel(address, devices));
	// Lampu menyala berurutan sekali saat panel muncul; setelah itu hanya lampu yang berubah yang berkedip.
	let sweep = $state(true);
	onMount(() => { const t = setTimeout(() => sweep = false, 1600); return () => clearTimeout(t); });
</script>

{#if panel}
	<section class="ports" aria-label="Alamat VPN">
		<div class="ports-caption">
			<span><strong>Alamat {panel.network}</strong> · {name}</span>
			<!-- Tanpa daftar perangkat, port kosong belum tentu kosong: jangan sebut jumlah atau alamat berikutnya. -->
			<span role="status">{#if known}{panel.used} terpakai · {panel.free} kosong{panel.next ? ` · berikutnya ${panel.next}` : ' · subnet penuh'}{:else}Pemakaian alamat belum diketahui{/if}</span>
		</div>
		<div class="ports-row" class:sweep>
			{#each panel.ports as port, i (port.address)}
				{#if port.device}
					{@const d = port.device}
					{@const state = deviceState(d, current)}
					{@const busy = state === 'online' && (rates[d.id] ?? 0) >= BUSY_RATE}
					{@const label = busy ? 'Online, ada trafik' : stateLabels[state]}
					<button class="port {state}" class:busy class:selected={selected === d.id} style="--i:{i}" title="{d.name} · {port.address} · {label}" aria-label="{d.name}, {port.address}, {label}" onclick={() => onselect(d.id)}>
						{#key busy ? 'busy' : state}<i></i>{/key}{port.label}
					</button>
				{:else}
					<span class="port" style="--i:{i}" title={known ? `${port.address} kosong` : port.address} aria-hidden="true"><i></i>{port.label}</span>
				{/if}
			{/each}
		</div>
		{#if known && panel.hidden}<p class="ports-more">{panel.hidden} perangkat lain ada di alamat yang lebih tinggi; lihat tabel di bawah.</p>{/if}
	</section>
{/if}
