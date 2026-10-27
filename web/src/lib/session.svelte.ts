import type { Me } from './api';

// Info pengguna yang sedang masuk; diisi oleh +layout.svelte.
export const session = $state<{ me: Me | null; initialized: boolean }>({
	me: null,
	initialized: false
});
