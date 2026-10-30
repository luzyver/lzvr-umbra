<script lang="ts">
	import { toasts, dismiss } from './toast.svelte';

	const dot = { ok: 'bg-[var(--green)]', warn: 'bg-[var(--accent)]', error: 'bg-[var(--red)]' };
</script>

<div class="toast-stack fixed right-4 bottom-4 z-50 flex w-80 flex-col gap-2" aria-live="polite">
	{#each toasts as t (t.id)}
		<button
			class="flex items-start gap-2.5 rounded-md border border-[var(--border)] bg-[var(--panel)] px-3.5 py-2.5 text-left text-sm text-[var(--text)] shadow-[0_12px_32px_-12px_#0000003d]"
			onclick={() => dismiss(t.id)}
		>
			<i class="mt-1.5 size-1.5 shrink-0 rounded-full {dot[t.kind]}"></i>{t.text}
		</button>
	{/each}
</div>
