<script lang="ts">
 let { name, size = 20 }: { name: string; size?: number } = $props();
 const paths: Record<string,string> = {
  overview:'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
  laptop:'M4 3h16v13H4z M4 16l-2 4h20l-2-4 M9 20h6',
  shield:'M12 2l8 4v6c0 6-8 10-8 10S4 18 4 12V6z M8 12l3 3 5-6',
  settings:'M3 7h2 M11 7h10 M3 17h10 M19 17h2 M11 7a3 3 0 1 1-6 0 3 3 0 1 1 6 0 M19 17a3 3 0 1 1-6 0 3 3 0 1 1 6 0',
  search:'M17 10a7 7 0 1 1-14 0 7 7 0 1 1 14 0 M15 15l6 6',
  plus:'M12 5v14 M5 12h14', arrow:'M5 12h14 M13 6l6 6-6 6',
  sun:'M16 12a4 4 0 1 1-8 0 4 4 0 1 1 8 0 M12 2v2 M12 20v2 M2 12h2 M20 12h2 M5 5l1 1 M18 18l1 1 M5 19l1-1 M18 6l1-1',
  moon:'M20 15A9 9 0 0 1 9 4a9 9 0 1 0 11 11',
  monitor:'M3 4h18v12H3z M8 20h8 M12 16v4',
  logout:'M9 3H4v18h5 M9 12h12 M16 7l5 5-5 5',
  server:'M3 3h18v7H3z M3 14h18v7H3z M7 7h.01 M7 18h.01 M13 7h4 M13 18h4',
  activity:'M3 12h4l3-8 4 16 3-8h4',
  menu:'M4 6h16 M4 12h16 M4 18h16', close:'M6 6l12 12 M6 18L18 6',
  book:'M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z',
  github:'M15 22v-4a4.8 4.8 0 0 0-1-3.5c3 0 6-2 6-5.5.08-1.25-.27-2.48-1-3.5.28-1.15.28-2.35 0-3.5 0 0-1 0-3 1.5-2.64-.5-5.36-.5-8 0C6 2 5 2 5 2c-.3 1.15-.3 2.35 0 3.5A5.403 5.403 0 0 0 4 9c0 3.5 3 5.5 6 5.5-.39.49-.68 1.05-.85 1.65-.17.6-.22 1.23-.15 1.85v4 M9 18c-4.51 2-5-2-7-2',
  external:'M7 17L17 7 M8 7h9v9',
  down:'M12 5v14 M6 13l6 6 6-6', up:'M12 19V5 M6 11l6-6 6 6'
 };
</script>
<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d={paths[name] ?? paths.overview}/></svg>
