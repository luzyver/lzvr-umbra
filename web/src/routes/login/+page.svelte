<script lang="ts">
	import Logo from '#lib/Logo.svelte';
	import { goto } from '$app/navigation';
	import { post, get, errMsg, type Me } from '#lib/api.ts';
	import { session } from '#lib/session.svelte.ts';

	let username = $state('');
	let password = $state('');
	let busy = $state(false);
	let error = $state('');
	let challenge = $state('');
	let code = $state('');

	let ok = $state(false);
	// Lampu pelat mengikuti langkah masuk; teks di formulir tetap menjadi sumber utama.
	const plate = $derived(ok ? 'ok' : busy ? 'busy' : error ? 'error' : challenge ? 'half' : 'idle');
	const plateLabel = $derived({ ok: 'Tersambung', busy: 'Memeriksa', error: 'Ditolak', half: 'Langkah 1 dari 2', idle: 'Belum masuk' }[plate]);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			if (challenge) {
				await post('/auth/verify', { challenge, code: code.trim() });
			} else {
				const result = await post<{ requires_totp?: boolean; challenge?: string }>('/auth/login', { username: username.trim(), password });
				if (result.requires_totp && result.challenge) {
					challenge = result.challenge;
					password = '';
					return;
				}
			}
			ok = true;
			session.me = await get<Me>('/auth/me');
			await goto(session.initialized ? '/' : '/setup');
		} catch (e) {
			ok = false;
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Masuk · Umbra</title></svelte:head>

<main class="login-screen">
	<div class="login-unit">
		<div class="ports login-plate" data-state={plate}>
			<div class="ports-caption"><Logo /><span aria-hidden="true">{plateLabel}</span></div>
			<div class="login-leds" aria-hidden="true">
				{#each Array(12) as _, i (i)}<span class="port" style="--i:{i}"><i></i></span>{/each}
			</div>
		</div>
		<form class="login-form" onsubmit={submit} oninput={() => error = ''}>
			<h1>{challenge ? 'Verifikasi dua langkah' : 'Masuk'}</h1>
			{#if challenge}
				<div>
					<label class="label" for="code">Kode verifikasi</label>
					<!-- svelte-ignore a11y_autofocus -->
					<input id="code" class="input font-mono" bind:value={code} autocomplete="one-time-code" aria-describedby="code-hint" autofocus required />
					<p class="hint" id="code-hint">Kode dari aplikasi autentikator, atau satu kode pemulihan.</p>
				</div>
			{:else}
				<div>
					<label class="label" for="u">Nama pengguna</label>
					<input id="u" class="input" bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" required />
				</div>
				<div>
					<label class="label" for="p">Kata sandi</label>
					<input id="p" class="input" type="password" bind:value={password} autocomplete="current-password" required />
				</div>
			{/if}
			{#if error}<p class="login-error" role="alert">{error}</p>{/if}
			<button class="btn-primary" disabled={busy}>{busy ? 'Memeriksa…' : challenge ? 'Verifikasi' : 'Masuk'}</button>
			{#if challenge}<button type="button" class="btn-ghost" onclick={() => { challenge = ''; code = ''; error = ''; }}>Kembali ke masuk</button>{/if}
		</form>
	</div>
</main>
