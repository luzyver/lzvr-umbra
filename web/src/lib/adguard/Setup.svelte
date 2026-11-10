<script lang="ts">
	import { post, errMsg, type AdGuardJob } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import ConnForm from './ConnForm.svelte';

	// View shown while AdGuard is not connected: install automatically or connect manually.
	let { job, ondone }: { job: AdGuardJob; ondone: () => void } = $props();

	let manual = $state(false);
	const running = $derived(job.state === 'running');

	async function install() {
		try {
			await post('/adguard/install');
			ondone();
		} catch (e) {
			toast(errMsg(e), 'error');
		}
	}
</script>

<div class="grid gap-4 md:grid-cols-2">
	<div class="card space-y-3 text-sm">
		<p class="font-medium">Install AdGuard Home</p>
		<p class="hint">
			The panel downloads AdGuard Home, runs the initial setup, creates an admin account, then points
			device DNS to the server's WireGuard IP. The AdGuard web interface is only open on localhost; all
			settings go through this panel.
		</p>
		{#if running}
			<p class="flex items-center gap-2">
				<span class="inline-block h-3 w-3 animate-spin rounded-full border-2 border-zinc-400 border-t-transparent"></span>
				{job.message || 'Installing…'}
			</p>
		{:else if job.state === 'error' && job.action === 'install'}
			<p class="text-red-600">Failed: {job.message}</p>
		{/if}
		{#if job.detected}
			<p class="rounded bg-amber-50 p-2 text-amber-800 dark:bg-amber-950 dark:text-amber-200">
				AdGuard Home was detected at 127.0.0.1:3000. If it is not set up yet, the button below
				runs its initial setup; if it is, connect manually.
			</p>
		{/if}
		{#if job.helper_available}
			<button class="btn-primary" disabled={running} onclick={install}>
				{running ? 'Installing…' : 'Install AdGuard'}
			</button>
		{:else}
			<p class="hint">
				Automatic installation needs a root helper that the latest installer sets up. Update Umbra
				(run update.sh) or re-run the installer from the release folder:
			</p>
			<pre class="overflow-x-auto rounded bg-zinc-100 p-3 font-mono text-xs dark:bg-zinc-950">sudo sh umbra-linux-amd64/install.sh --with-adguard umbra-linux-amd64/umbra</pre>
		{/if}
	</div>

	<div class="card space-y-3 text-sm">
		<p class="font-medium">Already have AdGuard Home?</p>
		<p class="hint">Connect your existing installation with its web address and admin account.</p>
		{#if manual}
			<ConnForm onsaved={ondone} />
		{:else}
			<button class="btn-ghost" disabled={running} onclick={() => (manual = true)}>Connect manually</button>
		{/if}
	</div>
</div>
