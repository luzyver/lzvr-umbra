// Small notifications in the corner of the screen.
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

// Show warnings from the API (for example an AdGuard failure) without cancelling the action.
export function warnAll(warnings: string[] | undefined) {
	for (const w of warnings ?? []) toast(w, 'warn', 7000);
}
