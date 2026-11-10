<script lang="ts">
	import { untrack } from 'svelte';
	import { flip } from 'svelte/animate';
	import type { ClientStatus } from './api';
	import { bytes, ago } from './format';
	import Icon from './Icon.svelte';

	let { clients, empty }: { clients: ClientStatus[]; empty: string } = $props();

	// Devices whose status just changed stay marked until hovered or focused.
	let seen = new Map<number, boolean>();
	let changed = $state(new Set<number>());
	$effect(() => {
		const next = new Set(untrack(() => changed));
		for (const c of clients) {
			const on = c.online && c.enabled;
			const before = seen.get(c.id);
			if (before !== undefined && before !== on) next.add(c.id);
			seen.set(c.id, on);
		}
		if (next.size !== untrack(() => changed.size)) changed = next;
	});
	function noticed(id: number) {
		if (!changed.has(id)) return;
		const next = new Set(changed);
		next.delete(id);
		changed = next;
	}

	// Tag number = last octet of the VPN address, like a room number on a key tag.
	function tagNo(address: string) {
		const ip = address.split('/')[0];
		const sep = ip.includes(':') ? ':' : '.';
		return { sep, no: ip.slice(ip.lastIndexOf(sep) + 1) || ip };
	}
	const reduce = typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
</script>

{#if !clients.length}
	<div class="rack-empty">{empty}</div>
{:else}
	<ul class="rack" aria-label="Devices">
		{#each clients as c (c.id)}
			{@const on = c.online && c.enabled}
			{@const t = tagNo(c.address)}
			<li animate:flip={{ duration: reduce ? 0 : 420 }}>
				<a
					class="key-tag"
					class:online={on}
					class:disabled={!c.enabled}
					class:changed={changed.has(c.id)}
					href="/activity?client={c.id}"
					title="View activity for {c.name} ({c.address})"
					onmouseenter={() => noticed(c.id)}
					onfocus={() => noticed(c.id)}
				>
					<i class="key-hole" aria-hidden="true"></i>
					<span class="key-no"><small>{t.sep}</small>{t.no}</span>
					<span class="key-name">{c.name}</span>
					<span class="key-meta">
						<i class="status-dot" aria-hidden="true"></i>
						{!c.enabled ? 'Disabled' : on ? 'Online' : ago(c.last_handshake)}
					</span>
					<span class="key-traffic">
						<span><Icon name="down" size={12} /><span class="sr-only">Download</span>{bytes(c.tx_bytes)}</span>
						<span><Icon name="up" size={12} /><span class="sr-only">Upload</span>{bytes(c.rx_bytes)}</span>
					</span>
				</a>
			</li>
		{/each}
	</ul>
{/if}
