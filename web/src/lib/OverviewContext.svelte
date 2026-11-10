<script lang="ts">
	import type { ServerConfig, AdGuardStatus } from './api';
	let { server, serverError, adguard, showDns, totals }: {
		server: ServerConfig | null; serverError: string; adguard: AdGuardStatus | null;
		showDns: boolean; totals: string;
	} = $props();
</script>

<aside class="network-context" aria-label="Network information">
	{#if server}
		<dl class="context-facts">
			<div><dt>Endpoint</dt><dd>{server.endpoint_host || 'Not set'}</dd></div>
			<div><dt>Port</dt><dd>UDP {server.endpoint_port}</dd></div>
			<div><dt>Outbound interface</dt><dd>{server.out_interface || '—'}</dd></div>
			{#if showDns && adguard?.reachable && adguard.status}
				<div><dt>DNS address</dt><dd>{adguard.status.dns_addresses.map(a => `${a}:${adguard.status?.dns_port}`).join(', ') || '—'}</dd></div>
			{/if}
			<div><dt>Total in / out</dt><dd>{totals}</dd></div>
		</dl>
	{:else}<p class="context-message" role="status">{serverError || 'Loading server…'}</p>{/if}
</aside>
