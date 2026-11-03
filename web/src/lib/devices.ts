export type DeviceState = 'online' | 'offline' | 'disabled' | 'unknown';
export type DeviceFilter = 'all' | Exclude<DeviceState, 'unknown'>;
export type DeviceRow = {
	id: number;
	name: string;
	address: string;
	enabled: boolean;
	online?: boolean;
	rx_bytes?: number;
	tx_bytes?: number;
	last_handshake?: string | null;
};

export function deviceState(device: DeviceRow, current: boolean): DeviceState {
	if (!device.enabled) return 'disabled';
	if (!current || device.online === undefined) return 'unknown';
	return device.online ? 'online' : 'offline';
}

export const stateLabels: Record<DeviceState, string> = {
	online: 'Online', offline: 'Offline', disabled: 'Nonaktif', unknown: 'Belum diketahui'
};

// Keep rows in a predictable address order, even when live status changes.
const addressOrder = new Intl.Collator('en', { numeric: true });
export function selectDevices<T extends DeviceRow>(devices: T[], query: string, filter: DeviceFilter, current: boolean): T[] {
	const term = query.trim().toLocaleLowerCase();
	return devices.filter(d =>
		(!term || `${d.name} ${d.address}`.toLocaleLowerCase().includes(term)) &&
		(filter === 'all' || deviceState(d, current) === filter)
	).sort((a, b) => addressOrder.compare(a.address, b.address) || a.id - b.id);
}
