import type { Me } from './api';

// Info about the signed-in user; filled in by +layout.svelte.
export const session = $state<{ me: Me | null; initialized: boolean }>({
	me: null,
	initialized: false
});
