import { test } from 'node:test';
import assert from 'node:assert/strict';
import { deviceState, selectDevices } from '../src/lib/devices.ts';

const devices = [
	{ id: 1, name: 'Laptop kantor', address: '10.8.0.10/32', enabled: true, online: true },
	{ id: 2, name: 'Ponsel', address: '10.8.0.2/32', enabled: true, online: false },
	{ id: 3, name: 'Arsip', address: '10.8.0.3/32', enabled: false, online: true },
	{ id: 4, name: 'Router', address: '10.8.0.4/32', enabled: true, online: true },
	{ id: 5, name: 'Baru', address: '10.8.0.5/32', enabled: true }
];

test('disabled takes precedence over an old online snapshot', () => {
	assert.equal(deviceState(devices[2], true), 'disabled');
});
test('missing or stale telemetry is unknown, never offline or online', () => {
	assert.equal(deviceState(devices[4], true), 'unknown');
	assert.equal(deviceState(devices[0], false), 'unknown');
	assert.equal(deviceState(devices[1], false), 'unknown');
});
test('offline filter excludes disabled and unobserved devices', () => {
	assert.deepEqual(selectDevices(devices, '', 'offline', true).map(d => d.id), [2]);
	assert.deepEqual(selectDevices(devices, '', 'offline', false), []);
});
test('search trims spaces and matches names without case sensitivity or IP addresses', () => {
	assert.deepEqual(selectDevices(devices, '  LAPTOP  ', 'all', true).map(d => d.id), [1]);
	assert.deepEqual(selectDevices(devices, '10.8.0.4', 'online', true).map(d => d.id), [4]);
});
test('numeric IP order stays stable across live updates and never mutates the source', () => {
	assert.deepEqual(selectDevices(devices, '', 'all', true).map(d => d.id), [2, 3, 4, 5, 1]);
	assert.deepEqual(devices.map(d => d.id), [1, 2, 3, 4, 5]);
	assert.deepEqual(selectDevices(devices, '', 'all', false).map(d => d.id), [2, 3, 4, 5, 1]);
});
