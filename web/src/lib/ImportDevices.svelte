<script lang="ts">
 import { get, errMsg, type ServerConfig } from './api.ts';
 import { toast, warnAll } from './toast.svelte.ts';
 import Modal from './Modal.svelte';

 let { open = $bindable(false), onimported }: { open: boolean; onimported: () => Promise<void> } = $props();
 type Row = { name: string; address: string; public_key: string; enabled: boolean; has_private_key: boolean; reason?: string };
 type Preview = { rows: Row[]; new: number; skipped: number; signature: string; server_address: string; server_listen_port: number; server_key_changes: boolean; warnings: string[] };
 let source = $state<'wgconf' | 'wgui'>('wgconf');
 let content = $state('');
 let selected = $state<File[]>([]);
 let adopt = $state(false);
 let preview = $state<Preview | null>(null);
 let error = $state('');
 let busy = $state(false);

 $effect(() => { if (open) get<ServerConfig>('/server').then((s) => { adopt = !s.initialized; }).catch(() => {}); });

 function filesChanged(e: Event) { selected = Array.from((e.currentTarget as HTMLInputElement).files ?? []); preview = null; }
 function form() {
  const data = new FormData();
  data.set('source', source); data.set('adopt_server', String(adopt));
  if (source === 'wgconf') data.set('text', content);
  for (const f of selected) data.append('files', f);
  return data;
 }
 async function send(path: string, data: FormData) {
  const res = await fetch('/api/import/' + path, { method: 'POST', body: data, credentials: 'same-origin', headers: { 'X-Umbra-Request': '1' } });
  const body = await res.json();
  if (!res.ok) throw new Error(body.error ?? 'Impor gagal.');
  return body;
 }
 async function inspect() {
  busy = true; error = ''; preview = null;
  try { preview = await send('preview', form()) as Preview; }
  catch (e) { error = errMsg(e); }
  finally { busy = false; }
 }
 async function commit() {
  if (!preview) return;
  busy = true; error = '';
  try {
   const result = await send(`commit?signature=${encodeURIComponent(preview.signature)}`, form()) as { imported: number; warnings: string[] };
   warnAll(result.warnings); toast(`${result.imported} perangkat diimpor.`); open = false; await onimported();
  } catch (e) { error = errMsg(e); preview = null; }
  finally { busy = false; }
 }
</script>

<Modal bind:open title="Impor perangkat" wide>
 <div class="space-y-4 text-sm">
  <div>
   <label class="label" for="import-source">Sumber</label>
   <select id="import-source" class="input" bind:value={source} onchange={() => { preview = null; selected = []; }}>
    <option value="wgconf">Konfigurasi server wg0.conf</option>
    <option value="wgui">Database wireguard-ui</option>
   </select>
  </div>
  {#if source === 'wgconf'}
   <div><label class="label" for="import-text">Tempel wg0.conf</label><textarea id="import-text" class="input min-h-32 font-mono text-xs" bind:value={content} oninput={() => (preview = null)}></textarea></div>
   <p class="hint">Atau unggah berkas .conf. Kunci privat perangkat tidak tersedia dari konfigurasi server.</p>
  {:else}
   <p class="hint">Unggah arsip .zip/.tar.gz dari folder db, atau pilih beberapa berkas JSON perangkat.</p>
  {/if}
  <input class="input" type="file" multiple={source === 'wgui'} accept={source === 'wgui' ? '.zip,.gz,.json' : '.conf,.txt'} onchange={filesChanged} />
  <label class="flex items-center gap-2"><input type="checkbox" bind:checked={adopt} onchange={() => (preview = null)} /> Gunakan kunci dan port server dari sumber</label>
  <button class="btn-ghost" disabled={busy} onclick={inspect}>{busy ? 'Memeriksa…' : 'Lihat pratinjau'}</button>
  {#if error}<p class="text-[var(--red)]">{error}</p>{/if}
  {#if preview}
   <p>{preview.new} baru · {preview.skipped} dilewati · subnet {preview.server_address}</p>
   {#if preview.server_key_changes}<p class="text-[var(--accent-ink)]">Kunci server berbeda. Periksa dampaknya sebelum menyimpan.</p>{/if}
   {#each preview.warnings as warning}<p class="text-[var(--accent-ink)]">{warning}</p>{/each}
   <div class="max-h-64 overflow-auto"><table class="data-table"><thead><tr><th>Perangkat</th><th>Alamat</th><th>Status</th></tr></thead><tbody>
    {#each preview.rows as row}<tr><td>{row.name}</td><td class="font-mono">{row.address}</td><td>{row.reason || 'Baru'}{#if !row.has_private_key && !row.reason} · tanpa kunci privat{/if}</td></tr>{/each}
   </tbody></table></div>
  {/if}
 </div>
 {#snippet footer()}
  <button class="btn-ghost" onclick={() => (open = false)}>Batal</button>
  {#if preview}<button class="btn-primary" disabled={busy || (preview.new === 0 && !adopt)} onclick={commit}>{busy ? 'Menyimpan…' : 'Impor perangkat'}</button>{/if}
 {/snippet}
</Modal>
