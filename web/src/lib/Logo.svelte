<script lang="ts">
 let { wordmark = true }: { wordmark?: boolean } = $props();
</script>
<span class="umbra-logo">
 <svg viewBox="0 0 64 64" aria-hidden="true" class="umbra-mark"><path d="M50.5 47.3A24 24 0 1 1 36.8 8.5A21 21 0 0 0 50.5 47.3Z"/><circle cx="50" cy="15" r="4"/></svg>
 {#if wordmark}<span>umbra</span>{/if}
</span>
