const units = ['B', 'KB', 'MB', 'GB', 'TB'];

export function bytes(n: number): string {
	if (!n || n < 0) return '0 B';
	let i = 0;
	while (n >= 1024 && i < units.length - 1) {
		n /= 1024;
		i++;
	}
	return `${n < 10 && i > 0 ? n.toFixed(1) : Math.round(n)} ${units[i]}`;
}

export function rate(bps: number): string {
	return bytes(bps) + '/s';
}

export function ago(iso: string | null | undefined, now = Date.now()): string {
	if (!iso) return 'never';
	const s = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000));
	if (s < 10) return 'just now';
	if (s < 60) return `${s}s ago`;
	const m = Math.floor(s / 60);
	if (m < 60) return `${m}m ago`;
	const h = Math.floor(m / 60);
	if (h < 24) return `${h}h ago`;
	return `${Math.floor(h / 24)}d ago`;
}

export function date(iso: string): string {
	return new Date(iso).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' });
}

// List separated by commas, spaces or newlines -> array without empty items.
export function splitList(s: string): string[] {
	return s
		.split(/[\s,]+/)
		.map((x) => x.trim())
		.filter(Boolean);
}

// Duration in seconds -> "2h 5m", "45s".
export function duration(sec: number): string {
	if (!sec || sec < 0) return '–';
	const d = Math.floor(sec / 86400);
	const h = Math.floor((sec % 86400) / 3600);
	const m = Math.floor((sec % 3600) / 60);
	if (d) return `${d}d ${h}h`;
	if (h) return `${h}h ${m}m`;
	if (m) return `${m}m`;
	return `${sec}s`;
}
