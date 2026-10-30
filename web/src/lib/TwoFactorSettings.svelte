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
			toast('Verifikasi dua langkah aktif.');
		} catch (e) { error = errMsg(e); }
		finally { busy = false; }
	}

	async function disable(e: SubmitEvent) {
		e.preventDefault(); busy = true; error = '';
		try {
			await post('/auth/totp/disable', { password, code: code.trim() });
			password = ''; code = '';
			await load();
			toast('Verifikasi dua langkah dinonaktifkan.');
		} catch (e) { error = errMsg(e); }
		finally { busy = false; }
	}

	function closeRecovery() { recoveryOpen = false; recovery = []; }
</script>

<div class="mt-4 max-w-md space-y-4">
	<h3 class="font-medium">Verifikasi dua langkah</h3>
	{#if !status}<p class="hint">Memuat…</p>
	{:else if status.enabled}
		<p class="hint">Aktif. Sisa kode pemulihan: {status.recovery_remaining}.</p>
		<form class="space-y-3" onsubmit={disable}>
			<div><label class="label" for="totp-password">Kata sandi</label><input id="totp-password" class="input" type="password" autocomplete="current-password" bind:value={password} required /></div>
			<div><label class="label" for="totp-disable-code">Kode autentikator atau pemulihan</label><input id="totp-disable-code" class="input font-mono" autocomplete="one-time-code" bind:value={code} required /></div>
			<button class="btn-ghost" disabled={busy}>Nonaktifkan verifikasi dua langkah</button>
		</form>
	{:else if setup}
		<p class="hint">Pindai kode QR dengan aplikasi autentikator, atau masukkan kunci ini secara manual.</p>
		<img src={setup.qr} alt="Kode QR untuk aplikasi autentikator" width="192" height="192" />
		<p class="break-all font-mono text-sm select-all">{setup.secret}</p>
		<form class="space-y-3" onsubmit={enable}>
			<div><label class="label" for="totp-enable-code">Kode 6 digit</label><input id="totp-enable-code" class="input font-mono" inputmode="numeric" autocomplete="one-time-code" maxlength="6" bind:value={code} required /></div>
			<button class="btn-primary" disabled={busy}>Aktifkan</button>
			<button type="button" class="btn-ghost" onclick={() => { setup = null; code = ''; }}>Batal</button>
		</form>
	{:else}
		<p class="hint">Minta kode dari aplikasi autentikator setiap kali masuk.</p>
		<button class="btn-primary" disabled={busy} onclick={start}>Siapkan verifikasi dua langkah</button>
	{/if}
	{#if error}<p class="text-sm text-[var(--red)]" role="alert">{error}</p>{/if}
</div>

<Modal bind:open={recoveryOpen} title="Kode pemulihan">
	<p class="hint mb-3">Simpan delapan kode ini sekarang. Kode hanya ditampilkan sekali dan masing-masing dapat dipakai satu kali.</p>
	<div class="grid grid-cols-2 gap-2 font-mono text-sm">{#each recovery as item}<span class="select-all break-all">{item}</span>{/each}</div>
	<button class="btn-primary mt-5" onclick={closeRecovery}>Sudah disimpan</button>
</Modal>
