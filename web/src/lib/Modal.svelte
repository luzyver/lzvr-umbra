<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		open = $bindable(false),
		title,
		children,
		footer,
		wide = false
	}: { open: boolean; title: string; children: Snippet; footer?: Snippet; wide?: boolean } =
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
	onclose={() => (open = false)}
	class="m-auto w-full rounded-lg border border-zinc-200 bg-white p-0 text-zinc-900 backdrop:bg-black/40 dark:border-zinc-800 dark:bg-zinc-900 dark:text-zinc-100 {wide
		? 'max-w-2xl'
		: 'max-w-md'}"
>
	{#if open}
		<div class="flex items-center justify-between border-b border-zinc-200 px-5 py-3 dark:border-zinc-800">
			<h2 class="font-semibold">{title}</h2>
			<button class="rounded p-1 text-zinc-500 hover:bg-zinc-100 dark:hover:bg-zinc-800" aria-label="Tutup" onclick={() => (open = false)}>✕</button>
		</div>
		<div class="px-5 py-4">{@render children()}</div>
		{#if footer}
			<div class="flex justify-end gap-2 border-t border-zinc-200 px-5 py-3 dark:border-zinc-800">
				{@render footer()}
			</div>
		{/if}
	{/if}
</dialog>
