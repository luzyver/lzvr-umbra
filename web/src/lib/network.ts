export type Port<T> = { address: string; label: string; device?: T };
export type PortPanel<T> = { network: string; used: number; free: number; next: string | null; ports: Port<T>[]; hidden: number };

/** IPv4 as an unsigned integer; a trailing /prefix is ignored. */
export function parseIPv4(value: string): number | null {
	const parts = value.split('/')[0].split('.');
	if (parts.length !== 4) return null;
	let n = 0;
	for (const part of parts) {
		if (!/^\d{1,3}$/.test(part) || Number(part) > 255) return null;
		n = n * 256 + Number(part);
	}
	return n;
}

export function formatIPv4(n: number): string {
	return [n >>> 24, (n >>> 16) & 255, (n >>> 8) & 255, n & 255].join('.');
}

/**
 * Lays the server subnet out as numbered ports, in the order the server hands
 * addresses out: every host address except the server's own. Shows whole rows
 * up to the last used address, at most `limit` ports.
 */
export function portPanel<T extends { address: string }>(cidr: string, devices: T[], row = 24, limit = 96): PortPanel<T> | null {
	const server = parseIPv4(cidr);
	const prefix = Number(cidr.split('/')[1]);
	if (server === null || !Number.isInteger(prefix) || prefix < 8 || prefix > 30) return null;
	const size = 2 ** (32 - prefix);
	const network = server - (server % size);
	const first = network + 1, last = network + size - 2;
	const serverIsHost = server >= first && server <= last;
	const hosts = size - 2 - (serverIsHost ? 1 : 0);
	const used = new Map<number, T>();
	for (const d of devices) {
		const a = parseIPv4(d.address);
		if (a !== null && a >= first && a <= last && a !== server && !used.has(a)) used.set(a, d);
	}
	const index = (a: number) => a - first - (serverIsHost && a > server ? 1 : 0);
	const lastUsed = Math.max(-1, ...[...used.keys()].map(index));
	const count = Math.min(hosts, limit, Math.max(row, Math.ceil((lastUsed + 1) / row) * row));
	const label = (a: number) => prefix >= 24 ? `.${a & 255}` : `.${(a >>> 8) & 255}.${a & 255}`;
	const ports: Port<T>[] = [];
	let next: number | null = null;
	for (let a = first; a <= last && (ports.length < count || next === null); a++) {
		if (a === server) continue;
		const device = used.get(a);
		if (!device && next === null) next = a;
		if (ports.length < count) ports.push({ address: formatIPv4(a), label: label(a), device });
	}
	return {
		network: `${formatIPv4(network)}/${prefix}`, used: used.size, free: hosts - used.size,
		next: next === null ? null : formatIPv4(next), ports,
		hidden: [...used.keys()].filter(a => index(a) >= count).length
	};
}
