// Notifikasi kecil di pojok layar.
export type Toast = { id: number; kind: 'ok' | 'error' | 'warn'; text: string };

let next = 1;
export const toasts = $state<Toast[]>([]);

export function toast(text: string, kind: Toast['kind'] = 'ok', ms = 4000) {
	const id = next++;
	toasts.push({ id, kind, text });
	setTimeout(() => dismiss(id), kind === 'error' ? ms * 2 : ms);
}

export function dismiss(id: number) {
	const i = toasts.findIndex((t) => t.id === id);
	if (i >= 0) toasts.splice(i, 1);
}

// Tampilkan peringatan dari API (misalnya AdGuard gagal) tanpa membatalkan aksi.
export function warnAll(warnings: string[] | undefined) {
	for (const w of warnings ?? []) toast(w, 'warn', 7000);
}
