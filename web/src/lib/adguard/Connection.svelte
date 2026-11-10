<script lang="ts">
	import { get, post, del, errMsg, type AdGuardConn } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import ConnForm from './ConnForm.svelte';

	let { onchange }: { onchange: () => void } = $props();

	let conn = $state<AdGuardConn | null>(null);
	let editing = $state(false);
	let busy = $state(false);

	get<AdGuardConn>('/adguard/connection')
		.then((r) => (conn = r))
		.catch((e) => toast(errMsg(e), 'error'));

	const sources = { db: 'stored in the panel', env: 'from umbra.env', none: '-' };

	async function disconnect() {
		if (!confirm('Disconnect the panel from AdGuard? AdGuard Home keeps running and device DNS is not changed.')) return;
		busy = true;
		try {
			const r = await del<{ enabled: boolean }>('/adguard/connection');
			toast(r?.enabled ? 'Panel connection removed, but umbra.env still contains AdGuard settings.' : 'AdGuard disconnected.');
			onchange();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	async function uninstall() {
		if (!confirm('Uninstall AdGuard Home from the server? Device DNS is reset to the server default DNS.')) return;
		busy = true;
		try {
			await post('/adguard/uninstall');
			toast('Uninstalling AdGuard…');
			onchange();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

{#if conn}
	<div class="grid gap-4 md:grid-cols-2">
		<div class="card space-y-3 text-sm">
			<p class="font-medium">Connection</p>
			<dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
				<dt class="text-zinc-500">Address</dt>
				<dd class="font-mono text-xs">{conn.url}</dd>
				<dt class="text-zinc-500">Username</dt>
				<dd>{conn.user || '-'}</dd>
				<dt class="text-zinc-500">Source</dt>
				<dd>{sources[conn.source]}</dd>
				<dt class="text-zinc-500">Installed by panel</dt>
				<dd>{conn.managed ? 'yes' : 'no'}</dd>
			</dl>
			{#if editing}
				<ConnForm url={conn.url} user={conn.user} hasPassword={conn.has_password} onsaved={onchange} />
			{:else}
				<div class="flex flex-wrap gap-2">
					<button class="btn-ghost" onclick={() => (editing = true)}>Edit connection</button>
					{#if conn.source === 'db'}
						<button class="btn-ghost" disabled={busy} onclick={disconnect}>Disconnect</button>
					{/if}
				</div>
			{/if}
			{#if conn.source === 'env'}
				<p class="hint">Saving here overrides the values in umbra.env without a restart.</p>
			{/if}
		</div>

		{#if conn.managed}
			<div class="card space-y-3 text-sm">
				<p class="font-medium">Uninstall AdGuard Home</p>
				<p class="hint">
					Stops and removes the AdGuard Home installed by the panel, then resets device DNS
					to the server default DNS.
				</p>
				<button class="btn-danger" disabled={busy} onclick={uninstall}>Uninstall AdGuard</button>
			</div>
		{/if}
	</div>
{:else}
	<p class="hint">Loading…</p>
{/if}
