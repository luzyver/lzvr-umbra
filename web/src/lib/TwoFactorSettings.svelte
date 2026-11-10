<script lang="ts">
	import { get, post, errMsg } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import Modal from '#lib/Modal.svelte';

	type Status = { enabled: boolean; recovery_remaining: number };
	type Setup = { secret: string; qr: string };
	let status = $state<Status | null>(null);
	let setup = $state<Setup | null>(null);
	let code = $state('');
	let password = $state('');
	let busy = $state(false);
	let error = $state('');
	let recovery = $state<string[]>([]);
	let recoveryOpen = $state(false);
	$effect(() => { if (!recoveryOpen) recovery = []; });

	async function load() {
		try { status = await get<Status>('/auth/totp'); }
		catch (e) { error = errMsg(e); }
	}
	load();

	async function start() {
		busy = true; error = '';
		try { setup = await post<Setup>('/auth/totp/start'); }
		catch (e) { error = errMsg(e); }
		finally { busy = false; }
	}

	async function enable(e: SubmitEvent) {
		e.preventDefault(); busy = true; error = '';
		try {
			const result = await post<{ recovery_codes: string[] }>('/auth/totp/enable', { code: code.trim() });
			recovery = result.recovery_codes;
			recoveryOpen = true;
			setup = null; code = '';
			await load();
			toast('Two-factor authentication enabled.');
		} catch (e) { error = errMsg(e); }
		finally { busy = false; }
	}

	async function disable(e: SubmitEvent) {
		e.preventDefault(); busy = true; error = '';
		try {
			await post('/auth/totp/disable', { password, code: code.trim() });
			password = ''; code = '';
			await load();
			toast('Two-factor authentication disabled.');
		} catch (e) { error = errMsg(e); }
		finally { busy = false; }
	}

	function closeRecovery() { recoveryOpen = false; recovery = []; }
</script>

<div class="mt-4 max-w-md space-y-4">
	<h3 class="font-medium">Two-factor authentication</h3>
	{#if !status}<p class="hint">Loading…</p>
	{:else if status.enabled}
		<p class="hint">Enabled. Recovery codes remaining: {status.recovery_remaining}.</p>
		<form class="space-y-3" onsubmit={disable}>
			<div><label class="label" for="totp-password">Password</label><input id="totp-password" class="input" type="password" autocomplete="current-password" bind:value={password} required /></div>
			<div><label class="label" for="totp-disable-code">Authenticator or recovery code</label><input id="totp-disable-code" class="input font-mono" autocomplete="one-time-code" bind:value={code} required /></div>
			<button class="btn-ghost" disabled={busy}>Disable two-factor authentication</button>
		</form>
	{:else if setup}
		<p class="hint">Scan the QR code with an authenticator app, or enter this key manually.</p>
		<img src={setup.qr} alt="QR code for the authenticator app" width="192" height="192" />
		<p class="break-all font-mono text-sm select-all">{setup.secret}</p>
		<form class="space-y-3" onsubmit={enable}>
			<div><label class="label" for="totp-enable-code">6-digit code</label><input id="totp-enable-code" class="input font-mono" inputmode="numeric" autocomplete="one-time-code" maxlength="6" bind:value={code} required /></div>
			<button class="btn-primary" disabled={busy}>Enable</button>
			<button type="button" class="btn-ghost" onclick={() => { setup = null; code = ''; }}>Cancel</button>
		</form>
	{:else}
		<p class="hint">Ask for a code from the authenticator app at every sign-in.</p>
		<button class="btn-primary" disabled={busy} onclick={start}>Set up two-factor authentication</button>
	{/if}
	{#if error}<p class="text-sm text-[var(--red)]" role="alert">{error}</p>{/if}
</div>

<Modal bind:open={recoveryOpen} title="Recovery codes">
	<p class="hint mb-3">Save these eight codes now. They are shown only once and each can be used a single time.</p>
	<div class="grid grid-cols-2 gap-2 font-mono text-sm">{#each recovery as item}<span class="select-all break-all">{item}</span>{/each}</div>
	<button class="btn-primary mt-5" onclick={closeRecovery}>I have saved them</button>
</Modal>
