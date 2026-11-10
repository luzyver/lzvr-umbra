<script lang="ts">
	// Firewall with a 60-second trial: the rules are rolled back automatically unless confirmed.
	import { onDestroy } from 'svelte';
	import { get, post, errMsg, type FirewallInfo } from './api';
	import { toast } from './toast.svelte';

	let { onconfirmed }: { onconfirmed?: () => void } = $props();

	let info = $state<FirewallInfo | null>(null);
	let mss = $state(true);
	let busy = $state(false);
	let error = $state('');
	let now = $state(Date.now());
	let showPreview = $state(false);

	const timer = setInterval(() => {
		now = Date.now();
		if (info?.pending && new Date(info.pending.expires).getTime() <= now) load();
	}, 1000);
	onDestroy(() => clearInterval(timer));

	const left = $derived(
		info?.pending ? Math.max(0, Math.ceil((new Date(info.pending.expires).getTime() - now) / 1000)) : 0
	);

	async function load() {
		try {
			info = await get<FirewallInfo>('/firewall');
			if (!info.pending) mss = info.state.applied ? info.state.mss_clamp : true;
		} catch (e) {
			error = errMsg(e);
		}
	}
	load();

	async function act(fn: () => Promise<unknown>, ok?: string) {
		busy = true;
		error = '';
		try {
			await fn();
			if (ok) toast(ok);
			await load();
		} catch (e) {
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}

	const apply = () => act(() => post('/firewall/apply', { mss_clamp: mss }));
	const confirm = () =>
		act(async () => {
			await post('/firewall/confirm');
			onconfirmed?.();
		}, 'Firewall enabled and saved.');
	const remove = () => act(() => post('/firewall/remove'), 'Panel firewall rules deleted.');
</script>

{#if !info}
	<p class="hint">Loading…</p>
{:else}
	<div class="space-y-4">
		<p class="text-sm text-[var(--muted)]">
			The panel creates its own nftables table (<span class="font-mono">inet umbra</span>) for NAT and
			forwards <span class="font-mono">{info.rules.wg_interface}</span> →
			<span class="font-mono">{info.rules.out_interface}</span> traffic (subnet
			<span class="font-mono">{info.rules.subnet}</span>). Other rules on the server are left untouched.
		</p>

		<div class="flex items-center gap-2 text-sm">
			<i class="check-led {info.pending ? 'warn' : info.state.applied ? 'ok' : 'off'}" style="margin-top:0" aria-hidden="true"></i>
			{info.pending ? 'Trial' : info.state.applied ? 'Enabled' : 'Not installed'}
		</div>

		{#if info.pending}
			<div class="rounded border border-[var(--busy)] bg-[var(--accent-soft)] p-4">
				<p class="font-medium">Rules are on trial, {left} seconds left</p>
				<p class="hint mt-1">
					Try connecting a device or check that this panel still opens. If you do not confirm,
					the rules are rolled back automatically so you do not get locked out.
				</p>
				<div class="mt-3 flex flex-wrap gap-2">
					<button class="btn-primary" disabled={busy} onclick={confirm}>All good, confirm</button>
					<button class="btn-ghost" disabled={busy} onclick={remove}>Cancel</button>
				</div>
			</div>
		{:else}
			<label class="flex items-start gap-2 text-sm">
				<input type="checkbox" class="mt-0.5" bind:checked={mss} />
				<span>
					MSS clamping
					<span class="hint block">Recommended. Prevents some sites from stalling because of packet size.</span>
				</span>
			</label>
			<div class="flex flex-wrap gap-2">
				<button class="btn-primary" disabled={busy} onclick={apply}>
					{info.state.applied ? 'Reapply (60-second trial)' : 'Install firewall (60-second trial)'}
				</button>
				{#if info.state.applied}
					<button class="btn-ghost" disabled={busy} onclick={remove}>Delete rules</button>
				{/if}
				<button class="btn-ghost" type="button" onclick={() => (showPreview = !showPreview)}>
					{showPreview ? 'Hide' : 'View'} rules
				</button>
			</div>
		{/if}

		{#if showPreview}
			{#if info.preview_error}
				<p class="text-sm text-[var(--red)]">{info.preview_error}</p>
			{:else}
				<pre class="overflow-x-auto rounded border border-[var(--border)] bg-[var(--panel)] p-3 font-mono text-xs">{info.preview}</pre>
			{/if}
		{/if}

		{#if error}<p class="text-sm text-[var(--red)]" role="alert">{error}</p>{/if}
	</div>
{/if}
