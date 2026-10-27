<script lang="ts">
	import { toasts, dismiss } from './toast.svelte';

	const style = {
		ok: 'border-teal-600 bg-teal-50 text-teal-900 dark:bg-teal-950 dark:text-teal-100',
		warn: 'border-amber-500 bg-amber-50 text-amber-900 dark:bg-amber-950 dark:text-amber-100',
		error: 'border-red-600 bg-red-50 text-red-900 dark:bg-red-950 dark:text-red-100'
	};
</script>

<div class="fixed right-4 bottom-4 z-50 flex w-80 flex-col gap-2" aria-live="polite">
	{#each toasts as t (t.id)}
		<button
			class="rounded-lg border-l-4 px-4 py-3 text-left text-sm shadow-md {style[t.kind]}"
			onclick={() => dismiss(t.id)}
		>
			{t.text}
		</button>
	{/each}
</div>
