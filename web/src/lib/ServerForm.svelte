<script lang="ts">
	import { untrack } from 'svelte';
	import { put, errMsg, type ServerConfig, type SystemCheck } from './api';
	import { toast, warnAll } from './toast.svelte';

	let {
		server,
		check = null,
		submitLabel = 'Save',
		onsaved
	}: {
		server: ServerConfig;
		check?: SystemCheck | null;
		submitLabel?: string;
		onsaved: (s: ServerConfig, profileChanged: boolean) => void;
	} = $props();

	// Local copy so the form can be edited without touching the source data.
	let f = $state(untrack(() => ({ ...server })));
	let busy = $state(false);
	let error = $state('');

	$effect.pre(() => {
		// Build in a local variable: reading `f` here would make the effect trigger itself endlessly.
		const next = { ...server };
		// Prefill from the system check during first setup.
		if (!server.initialized && check) {
			if (!next.endpoint_host && check.public_ip) next.endpoint_host = check.public_ip;
			if (!next.out_interface && check.default_iface) next.out_interface = check.default_iface;
		}
		f = next;
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			const r = await put<{ server: ServerConfig; profile_changed: boolean; warnings: string[] }>(
				'/server',
				{
					interface: f.interface.trim(),
					address: f.address.trim(),
					listen_port: Number(f.listen_port),
					endpoint_host: f.endpoint_host.trim(),
					endpoint_port: Number(f.endpoint_port) || 0,
					mtu: Number(f.mtu) || 0,
					out_interface: f.out_interface.trim()
				}
			);
			warnAll(r.warnings);
			if (server.initialized) toast('Server settings saved and applied.');
			onsaved(r.server, r.profile_changed);
		} catch (e) {
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}
</script>

<form class="grid gap-4 sm:grid-cols-2" onsubmit={submit}>
	<div>
		<label class="label" for="sf-host">Public address (endpoint)</label>
		<input id="sf-host" class="input" bind:value={f.endpoint_host} placeholder="vpn.example.com" required />
		<p class="hint">Domain or IP that phones and laptops use to connect. If the domain is on Cloudflare, turn off the proxy (grey cloud), because WireGuard uses UDP.</p>
	</div>
	<div>
		<label class="label" for="sf-port">UDP port</label>
		<input id="sf-port" class="input" type="number" min="1" max="65535" bind:value={f.listen_port} required />
		<p class="hint">Open this port (UDP) in your VPS provider or router firewall.</p>
	</div>
	<div>
		<label class="label" for="sf-addr">Server address in VPN</label>
		<input id="sf-addr" class="input font-mono" bind:value={f.address} placeholder="10.8.0.1/24" required />
		<p class="hint">Devices get IPs from this subnet (10.8.0.2, 10.8.0.3, …).</p>
	</div>
	<div>
		<label class="label" for="sf-out">Internet interface</label>
		<input id="sf-out" class="input font-mono" bind:value={f.out_interface} placeholder="eth0" required />
		{#if check?.default_iface}
			<p class="hint">Detected: <span class="font-mono">{check.default_iface}</span></p>
		{/if}
	</div>
	<details class="sm:col-span-2">
		<summary class="cursor-pointer text-sm text-[var(--muted)]">Advanced settings</summary>
		<div class="mt-3 grid gap-4 sm:grid-cols-3">
			<div>
				<label class="label" for="sf-if">Interface name</label>
				<input id="sf-if" class="input font-mono" bind:value={f.interface} disabled={server.initialized} required />
				{#if server.initialized}<p class="hint">Cannot be changed after setup.</p>{/if}
			</div>
			<div>
				<label class="label" for="sf-eport">Public port</label>
				<input id="sf-eport" class="input" type="number" min="0" max="65535" bind:value={f.endpoint_port} />
				<p class="hint">0 = same as the UDP port. Set it if port forwarding uses a different port.</p>
			</div>
			<div>
				<label class="label" for="sf-mtu">Server MTU</label>
				<input id="sf-mtu" class="input" type="number" min="0" max="1500" bind:value={f.mtu} />
				<p class="hint">0 = automatic.</p>
			</div>
		</div>
	</details>
	{#if error}
		<p class="text-sm text-[var(--red)] sm:col-span-2" role="alert">{error}</p>
	{/if}
	<div class="sm:col-span-2">
		<button class="btn-primary" disabled={busy}>{busy ? 'Applying…' : submitLabel}</button>
	</div>
</form>
