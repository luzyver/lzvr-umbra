<script lang="ts">
	import { toasts, dismiss } from './toast.svelte';

	const dot = { ok: 'bg-green-500', warn: 'bg-amber-500', error: 'bg-red-500' };
</script>

<div class="fixed right-4 bottom-4 z-50 flex w-80 flex-col gap-2" aria-live="polite">
	{#each toasts as t (t.id)}
		<button
			class="flex items-start gap-2.5 rounded-md border border-[var(--border)] bg-[var(--panel)] px-3.5 py-2.5 text-left text-sm text-[var(--text)] shadow-lg shadow-black/5"
			onclick={() => dismiss(t.id)}
		>
			<i class="mt-1.5 size-1.5 shrink-0 rounded-full {dot[t.kind]}"></i>{t.text}
		</button>
	{/each}
</div>
