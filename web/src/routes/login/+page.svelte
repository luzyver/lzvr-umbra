<script lang="ts">
	import { goto } from '$app/navigation';
	import { post, get, errMsg, type Me } from '#lib/api.ts';
	import { session } from '#lib/session.svelte.ts';

	let username = $state('');
	let password = $state('');
	let busy = $state(false);
	let error = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			await post('/auth/login', { username: username.trim(), password });
			session.me = await get<Me>('/auth/me');
			await goto(session.initialized ? '/' : '/setup');
		} catch (e) {
			error = errMsg(e);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head><title>Masuk · Umbra</title></svelte:head>

<div class="flex min-h-screen items-center justify-center px-4">
	<form class="card w-full max-w-sm space-y-4" onsubmit={submit}>
		<div class="flex items-center gap-2">
			<img src="/favicon.svg" alt="" class="h-8 w-8" />
			<h1 class="text-lg font-semibold">Umbra</h1>
		</div>
		<div>
			<label class="label" for="u">Nama pengguna</label>
			<input id="u" class="input" bind:value={username} autocomplete="username" required />
		</div>
		<div>
			<label class="label" for="p">Kata sandi</label>
			<input id="p" class="input" type="password" bind:value={password} autocomplete="current-password" required />
		</div>
		{#if error}<p class="text-sm text-red-600">{error}</p>{/if}
		<button class="btn-primary w-full" disabled={busy}>{busy ? 'Masuk…' : 'Masuk'}</button>
	</form>
</div>
