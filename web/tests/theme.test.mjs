import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function boot(saved, dark = false, blocked = false) {
 const events = new EventTarget();
 const media = new EventTarget(); media.matches = dark;
 const root = { dataset: {}, style: {} };
 const storage = { value: saved, getItem() { if(blocked) throw Error('blocked'); return this.value; }, setItem(k,v) { if(blocked) throw Error('blocked'); this.value=v; }, removeItem() { this.value=null; } };
 const window = { matchMedia: () => media, addEventListener: events.addEventListener.bind(events), dispatchEvent: events.dispatchEvent.bind(events) };
 vm.runInNewContext(readFileSync(new URL('../static/theme.js', import.meta.url), 'utf8'), { window, document:{documentElement:root}, localStorage:storage, CustomEvent });
 return {root,storage, set(value) { events.dispatchEvent(new CustomEvent('umbra:set-theme',{detail:value})); }, os(value) {media.matches=value;media.dispatchEvent(new Event('change'));}, sync(value) {storage.value=value;events.dispatchEvent(Object.assign(new Event('storage'),{key:'umbra-theme'}));} };
}
test('system default follows OS changes',()=>{ const t=boot(null,true);assert.equal(t.root.dataset.theme,'dark');t.os(false);assert.equal(t.root.dataset.theme,'light'); });
test('manual selection persists and overrides OS until system reset',()=>{const t=boot('light',true);assert.equal(t.root.dataset.theme,'light');t.set('dark');assert.equal(t.storage.value,'dark');t.os(false);assert.equal(t.root.dataset.theme,'dark');t.set('system');assert.equal(t.root.dataset.theme,'light');assert.equal(t.storage.value,null);});
test('invalid storage falls back, cross-tab updates apply',()=>{const t=boot('invalid');assert.equal(t.root.dataset.theme,'light');t.sync('dark');assert.equal(t.root.dataset.theme,'dark');t.set('invalid');assert.equal(t.root.dataset.theme,'dark');});
test('blocked storage does not prevent switching',()=>{const t=boot(null,false,true);t.set('dark');t.os(false);assert.equal(t.root.dataset.theme,'dark');});
