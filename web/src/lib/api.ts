// Pembungkus fetch ke backend Go. Semua request tulis wajib membawa header
// X-Umbra-Request (perlindungan CSRF), jadi header itu selalu dikirim.

import { goto } from '$app/navigation';

export class ApiError extends Error {
	constructor(
		public status: number,
		message: string
	) {
		super(message);
	}
}

type Opts = { method?: string; body?: unknown; noRedirect?: boolean };

export async function api<T = unknown>(path: string, opts: Opts = {}): Promise<T> {
	const headers: Record<string, string> = { 'X-Umbra-Request': '1' };
	let body: BodyInit | undefined;
	if (opts.body !== undefined) {
		headers['Content-Type'] = 'application/json';
		body = JSON.stringify(opts.body);
	}
	let res: Response;
	try {
		res = await fetch('/api' + path, {
			method: opts.method ?? (body ? 'POST' : 'GET'),
			headers,
			body,
			credentials: 'same-origin'
		});
	} catch {
		throw new ApiError(0, 'Tidak bisa menghubungi panel. Periksa koneksi.');
	}
	if (res.status === 401 && !opts.noRedirect) {
		if (!location.pathname.startsWith('/login')) goto('/login');
		throw new ApiError(401, 'Sesi habis, silakan masuk lagi.');
	}
	if (res.status === 204) return undefined as T;
	const text = await res.text();
	let data: unknown = undefined;
	try {
		data = text ? JSON.parse(text) : undefined;
	} catch {
		data = text;
	}
	if (!res.ok) {
		const msg =
			data && typeof data === 'object' && 'error' in data
				? String((data as { error: unknown }).error)
				: `Permintaan gagal (${res.status})`;
		throw new ApiError(res.status, msg);
	}
	return data as T;
}

export const get = <T>(p: string, o: Omit<Opts, 'method' | 'body'> = {}) => api<T>(p, o);
export const post = <T>(p: string, body: unknown = {}) => api<T>(p, { method: 'POST', body });
export const put = <T>(p: string, body: unknown) => api<T>(p, { method: 'PUT', body });
export const del = <T>(p: string) => api<T>(p, { method: 'DELETE' });

export function errMsg(e: unknown): string {
	return e instanceof Error ? e.message : String(e);
}

// ---- Tipe data dari API ----

export type Me = { id: number; username: string; adguard: boolean; fake_wg: boolean };
export type SetupStatus = { needs_admin: boolean; initialized: boolean };

export type CheckItem = { id: string; label: string; status: 'ok' | 'warn' | 'fail'; detail: string };
export type SystemCheck = {
	items: CheckItem[];
	default_iface: string;
	local_ips: string[];
	public_ip: string;
	behind_nat: boolean;
};

export type ServerConfig = {
	interface: string;
	address: string;
	listen_port: number;
	endpoint_host: string;
	endpoint_port: number;
	mtu: number;
	out_interface: string;
	public_key: string;
	initialized: boolean;
};

export type Defaults = { dns: string[]; allowed_ips: string[]; keepalive: number; mtu: number };

export type FirewallInfo = {
	state: { applied: boolean; mss_clamp: boolean };
	pending: { rules: unknown; expires: string } | null;
	rules: { wg_interface: string; out_interface: string; subnet: string; mss_clamp: boolean };
	preview: string;
	preview_error?: string;
};

export type Client = {
	id: number;
	name: string;
	public_key: string;
	address: string;
	allowed_ips: string[];
	dns: string[];
	mtu: number;
	keepalive: number;
	enabled: boolean;
	adblock: boolean;
	created_at: string;
	updated_at: string;
};

export type ClientStatus = {
	id: number;
	name: string;
	address: string;
	enabled: boolean;
	online: boolean;
	endpoint?: string;
	rx_bytes: number;
	tx_bytes: number;
	last_handshake: string | null;
};

export type Status = {
	initialized: boolean;
	interface: string;
	online: number;
	total: number;
	rx_bytes: number;
	tx_bytes: number;
	clients: ClientStatus[];
	error?: string;
	time: string;
};

export type AdGuardStatus = {
	configured: boolean;
	reachable?: boolean;
	status?: {
		running: boolean;
		protection_enabled: boolean;
		version: string;
		dns_addresses: string[];
		dns_port: number;
	};
	error?: string;
};

export type AdGuardConn = {
	url: string;
	user: string;
	has_password: boolean;
	managed: boolean;
	source: 'db' | 'env' | 'none';
};

export type AdGuardJob = {
	action: string;
	state: 'idle' | 'running' | 'done' | 'error';
	message: string;
	helper_available: boolean;
	managed: boolean;
	detected: boolean;
};

export type AdGuardSync = { synced: number; removed: number; warnings: string[] };

// Pengaturan AdGuard per klien; field kosong berarti tidak diubah.
export type AdGuardClientSettings = {
	safebrowsing_enabled?: boolean;
	parental_enabled?: boolean;
	safe_search_enabled?: boolean;
	use_global_blocked_services?: boolean;
	blocked_services?: string[];
	upstreams?: string[];
};

// Log aktivitas WireGuard (dicatat panel dari handshake peer).
export type Activity = {
	id: number;
	client_id: number;
	client_name: string;
	kind: 'connect' | 'disconnect' | 'roam';
	endpoint?: string;
	detail?: string;
	duration: number;
	rx_bytes: number;
	tx_bytes: number;
	at: string;
};

export type ActivityPage = { items: Activity[]; more: boolean };
