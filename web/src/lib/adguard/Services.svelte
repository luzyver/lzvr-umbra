<script lang="ts">
	import { get, put, errMsg } from '#lib/api.ts';
	import { toast } from '#lib/toast.svelte.ts';
	import ServicePicker from './ServicePicker.svelte';

	let ids = $state<string[]>([]);
	let schedule = $state<unknown>(undefined);
	let loaded = $state(false);
	let busy = $state(false);

	get<{ ids: string[] | null; schedule?: unknown }>('/adguard/ctl/blocked_services/get')
		.then((r) => {
			ids = r.ids ?? [];
			schedule = r.schedule;
			loaded = true;
		})
		.catch((e) => toast(errMsg(e), 'error'));

	async function save() {
		busy = true;
		try {
			// Send the existing schedule back unchanged so it is not wiped.
			await put('/adguard/ctl/blocked_services/update', { ids, schedule });
			toast('Blocked services updated.');
		} catch (e) {
			toast(errMsg(e), 'error');
		} finally {
			busy = false;
		}
	}
</script>

<div class="card space-y-3">
	<div>
		<p class="font-medium">Block services</p>
		<p class="hint">
			Applies to all devices that use AdGuard, except devices with their own services list
			(set in the Clients tab).
		</p>
	</div>
	{#if loaded}
		<ServicePicker bind:selected={ids} disabled={busy} />
		<div class="flex justify-end">
			<button class="btn-primary" disabled={busy} onclick={save}>Save</button>
		</div>
	{:else}
		<p class="hint">Loading…</p>
	{/if}
</div>
