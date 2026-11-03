<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';

	let {
		open = $bindable(false),
		title,
		children,
		footer,
		wide = false,
		drawer = false
	}: { open: boolean; title: string; children: Snippet; footer?: Snippet; wide?: boolean; drawer?: boolean } =
		$props();

	let dialog: HTMLDialogElement | undefined = $state();

	$effect(() => {
		if (!dialog) return;
		if (open && !dialog.open) dialog.showModal();
		if (!open && dialog.open) dialog.close();
	});
</script>

<dialog
	bind:this={dialog}
	aria-label={title}
	onclose={() => (open = false)}
	class:device-drawer={drawer}
	class="m-auto w-full rounded-xl border p-0 {wide
		? 'max-w-2xl'
		: 'max-w-md'}"
>
	{#if open}
		<div class="dialog-header flex items-center justify-between border-b border-[var(--border)] px-5 py-3">
			<h2 class="min-w-0 break-words font-bold tracking-tight">{title}</h2>
			<button class="icon-btn" aria-label="Tutup" onclick={() => (open = false)}><Icon name="close" size={16} /></button>
		</div>
		<div class="dialog-body px-5 py-4">{@render children()}</div>
		{#if footer}
			<div class="dialog-footer flex justify-end gap-2 border-t border-[var(--border)] bg-[var(--panel-alt)]/40 px-5 py-3">
				{@render footer()}
			</div>
		{/if}
	{/if}
</dialog>
