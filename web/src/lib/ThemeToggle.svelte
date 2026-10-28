<script lang="ts">
 import { onMount } from 'svelte';
 import Icon from './Icon.svelte';
 const order = ['system', 'light', 'dark'];
 const labels: Record<string, string> = { system: 'Sistem', light: 'Terang', dark: 'Gelap' };
 const icons: Record<string, string> = { system: 'monitor', light: 'sun', dark: 'moon' };
 let preference = $state('system');
 onMount(() => {
  const sync = () => { preference = document.documentElement.dataset.themePreference ?? 'system'; };
  sync(); window.addEventListener('umbra:theme-changed', sync);
  return () => window.removeEventListener('umbra:theme-changed', sync);
 });
 function cycle() {
  const next = order[(order.indexOf(preference) + 1) % order.length];
  window.dispatchEvent(new CustomEvent('umbra:set-theme', { detail: next }));
 }
</script>
<button class="icon-btn" onclick={cycle} title="Tema: {labels[preference] ?? 'Sistem'}" aria-label="Tema: {labels[preference] ?? 'Sistem'}. Klik untuk mengganti."><Icon name={icons[preference] ?? 'monitor'} size={16}/></button>
