<script lang="ts">
	let {
		checked,
		disabled = false,
		label,
		onchange
	}: { checked: boolean; disabled?: boolean; label: string; onchange: (v: boolean) => void } =
		$props();
</script>

<button
	type="button"
	role="switch"
	aria-checked={checked}
	aria-label={label}
	title={label}
	{disabled}
	onclick={() => onchange(!checked)}
	class="relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition disabled:opacity-50 {checked
		? 'toggle-active'
		: 'bg-zinc-300 dark:bg-zinc-700'}"
>
	<span
		class="inline-block h-4 w-4 rounded-full bg-white shadow transition {checked
			? 'translate-x-4.5'
			: 'translate-x-0.5'}"
	></span>
</button>
