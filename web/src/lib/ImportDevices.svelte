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
  if (!res.ok) throw new Error(body.error ?? 'Import failed.');
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
   warnAll(result.warnings); toast(`${result.imported} devices imported.`); open = false; await onimported();
  } catch (e) { error = errMsg(e); preview = null; }
  finally { busy = false; }
 }
</script>

<Modal bind:open title="Import devices" wide>
 <div class="space-y-4 text-sm">
  <div>
   <label class="label" for="import-source">Source</label>
   <select id="import-source" class="input" bind:value={source} onchange={() => { preview = null; selected = []; }}>
    <option value="wgconf">Server config wg0.conf</option>
    <option value="wgui">wireguard-ui database</option>
   </select>
  </div>
  {#if source === 'wgconf'}
   <div><label class="label" for="import-text">Paste wg0.conf</label><textarea id="import-text" class="input min-h-32 font-mono text-xs" bind:value={content} oninput={() => (preview = null)}></textarea></div>
   <p class="hint">Or upload a .conf file. Device private keys are not available from the server config.</p>
  {:else}
   <p class="hint">Upload a .zip/.tar.gz archive of the db folder, or select several device JSON files.</p>
  {/if}
  <input class="input" type="file" multiple={source === 'wgui'} accept={source === 'wgui' ? '.zip,.gz,.json' : '.conf,.txt'} onchange={filesChanged} />
  <label class="flex items-center gap-2"><input type="checkbox" bind:checked={adopt} onchange={() => (preview = null)} /> Use the server key and port from the source</label>
  <button class="btn-ghost" disabled={busy} onclick={inspect}>{busy ? 'Checking…' : 'Preview'}</button>
  {#if error}<p class="text-[var(--red)]">{error}</p>{/if}
  {#if preview}
   <p>{preview.new} new · {preview.skipped} skipped · subnet {preview.server_address}</p>
   {#if preview.server_key_changes}<p class="text-[var(--accent-ink)]">The server key differs. Check the impact before saving.</p>{/if}
   {#each preview.warnings as warning}<p class="text-[var(--accent-ink)]">{warning}</p>{/each}
   <div class="max-h-64 overflow-auto"><table class="data-table"><thead><tr><th>Device</th><th>Address</th><th>Status</th></tr></thead><tbody>
    {#each preview.rows as row}<tr><td>{row.name}</td><td class="font-mono">{row.address}</td><td>{row.reason || 'New'}{#if !row.has_private_key && !row.reason} · no private key{/if}</td></tr>{/each}
   </tbody></table></div>
  {/if}
 </div>
 {#snippet footer()}
  <button class="btn-ghost" onclick={() => (open = false)}>Cancel</button>
  {#if preview}<button class="btn-primary" disabled={busy || (preview.new === 0 && !adopt)} onclick={commit}>{busy ? 'Saving…' : 'Import devices'}</button>{/if}
 {/snippet}
</Modal>
