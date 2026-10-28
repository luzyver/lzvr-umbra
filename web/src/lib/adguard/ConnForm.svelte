<script lang="ts">
	import { post, put, errMsg, type AdGuardSync } from '#lib/api.ts';
	import { toast, warnAll } from '#lib/toast.svelte.ts';

	let {
		url = 'http://127.0.0.1:3000',
		user = '',
		hasPassword = false,
		onsaved
	}: { url?: string; user?: string; hasPassword?: boolean; onsaved: () => void } = $props();

	// svelte-ignore state_referenced_locally
	let form = $state({ url, user, pass: '' });
	let busy = $state(false);

	async function test() {
		busy = true;
		try {
			const r = await post<{ status: { version: string } }>('/adguard/connection/test', form);
			toast(`Terhubung ke AdGuard Home ${r.status.version}.`);
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			const r = await put<AdGuardSync>('/adguard/connection', form);
			warnAll(r.warnings);
			toast(`AdGuard terhubung. ${r.synced} klien disinkronkan.`);
			onsaved();
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

<form class="space-y-3" onsubmit={save}>
	<div>
		<label class="label" for="ag-url">Alamat web AdGuard</label>
		<input id="ag-url" class="input w-full font-mono" bind:value={form.url} required placeholder="http://127.0.0.1:3000" />
	</div>
	<div class="grid gap-3 sm:grid-cols-2">
		<div>
			<label class="label" for="ag-user">Username</label>
			<input id="ag-user" class="input w-full" bind:value={form.user} autocomplete="off" />
		</div>
		<div>
			<label class="label" for="ag-pass">Kata sandi</label>
			<input
				id="ag-pass"
				type="password"
				class="input w-full"
				bind:value={form.pass}
				autocomplete="new-password"
				placeholder={hasPassword ? '(tidak diubah)' : ''}
			/>
		</div>
	</div>
	<div class="flex justify-end gap-2">
		<button type="button" class="btn-ghost" disabled={busy || !form.url} onclick={test}>Tes koneksi</button>
		<button class="btn-primary" disabled={busy}>Simpan & hubungkan</button>
	</div>
</form>
