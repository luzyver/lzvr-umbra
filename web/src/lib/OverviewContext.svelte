<script lang="ts">
	import type { ServerConfig, AdGuardStatus } from './api';
	import Icon from './Icon.svelte';
	let { server, serverError, adguard, dnsLabel, showDns }: {
		server: ServerConfig | null; serverError: string; adguard: AdGuardStatus | null;
		dnsLabel: string; showDns: boolean;
	} = $props();
</script>

<aside class="network-context" aria-label="Informasi jaringan">
	<section>
		<div class="context-heading"><h2><Icon name="server" size={17} />Server</h2><a href="/settings" aria-label="Pengaturan server"><Icon name="arrow" size={16} /></a></div>
		{#if server}
			<dl class="context-facts">
				<div class="context-endpoint"><dt>Endpoint</dt><dd>{server.endpoint_host || 'Belum diatur'}<span>UDP {server.endpoint_port}</span></dd></div>
				<div><dt>Interface</dt><dd>{server.interface}</dd></div>
				<div><dt>Alamat VPN</dt><dd>{server.address}</dd></div>
				<div><dt>Jalur keluar</dt><dd>{server.out_interface || '—'}</dd></div>
			</dl>
		{:else}<p class="context-message" role="status">{serverError || 'Memuat server…'}</p>{/if}
	</section>
	{#if showDns}
		<section>
			<div class="context-heading"><h2><Icon name="shield" size={17} />DNS & perlindungan</h2><a href="/adguard" aria-label="Pengaturan AdGuard"><Icon name="arrow" size={16} /></a></div>
			<p class="context-message">{dnsLabel}</p>
			{#if adguard?.reachable && adguard.status}
				<dl class="context-facts"><div><dt>Alamat DNS</dt><dd>{adguard.status.dns_addresses.join(', ') || '—'}</dd></div><div><dt>Port</dt><dd>{adguard.status.dns_port}</dd></div></dl>
			{/if}
		</section>
	{/if}
</aside>
