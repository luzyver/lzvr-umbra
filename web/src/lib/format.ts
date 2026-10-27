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
	if (!iso) return 'belum pernah';
	const s = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000));
	if (s < 10) return 'baru saja';
	if (s < 60) return `${s} detik lalu`;
	const m = Math.floor(s / 60);
	if (m < 60) return `${m} menit lalu`;
	const h = Math.floor(m / 60);
	if (h < 24) return `${h} jam lalu`;
	return `${Math.floor(h / 24)} hari lalu`;
}

export function date(iso: string): string {
	return new Date(iso).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });
}

// Daftar dipisah koma/spasi/baris baru -> array tanpa elemen kosong.
export function splitList(s: string): string[] {
	return s
		.split(/[\s,]+/)
		.map((x) => x.trim())
		.filter(Boolean);
}
