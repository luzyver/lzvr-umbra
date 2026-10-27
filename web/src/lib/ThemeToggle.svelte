<script lang="ts">
 import { onMount } from 'svelte';
 import Icon from './Icon.svelte';
 let preference = $state('system');
 let dark = $state(false);
 onMount(() => {
  const sync = () => { preference = document.documentElement.dataset.themePreference ?? 'system'; dark = document.documentElement.dataset.theme === 'dark'; };
  sync(); window.addEventListener('umbra:theme-changed', sync);
  return () => window.removeEventListener('umbra:theme-changed', sync);
 });
 function change(event: Event) { window.dispatchEvent(new CustomEvent('umbra:set-theme', { detail: (event.target as HTMLSelectElement).value })); }
</script>
<label class="theme-control"><Icon name={dark ? 'moon' : 'sun'} size={16}/><span class="sr-only">Tema tampilan</span><select aria-label="Tema tampilan" value={preference} onchange={change}><option value="system">Sistem</option><option value="dark">Dark</option><option value="light">Light</option></select></label>
