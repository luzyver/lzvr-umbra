// Run before styles/app hydration; external file works with script-src 'self'.
(() => {
 const root = document.documentElement;
 const media = window.matchMedia('(prefers-color-scheme: dark)');
 const valid = value => value === 'dark' || value === 'light';
 function read() { try { const value = localStorage.getItem('umbra-theme'); return valid(value) ? value : 'system'; } catch { return 'system'; } }
 let preference = read();
 function apply() {
  const theme = preference === 'system' ? (media.matches ? 'dark' : 'light') : preference;
  root.dataset.theme = theme;
  root.dataset.themePreference = preference;
  root.style.colorScheme = theme;
  window.dispatchEvent(new CustomEvent('umbra:theme-changed'));
 }
 window.addEventListener('umbra:set-theme', event => {
  if (!valid(event.detail) && event.detail !== 'system') return;
  preference = event.detail;
  try { if (preference === 'system') localStorage.removeItem('umbra-theme'); else localStorage.setItem('umbra-theme', preference); } catch { /* In-memory preference still works. */ }
  apply();
 });
 media.addEventListener('change', () => { if (preference === 'system') apply(); });
 window.addEventListener('storage', event => { if (event.key === 'umbra-theme' || event.key === null) { preference = read(); apply(); } });
 apply();
})();
