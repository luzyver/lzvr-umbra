import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';

// Build output goes to internal/ui/dist and is embedded into the Go binary.
const out = '../internal/ui/dist';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			adapter: adapter({ pages: out, assets: out, fallback: 'index.html' })
		})
	],
	server: {
		// npm run dev: the API is proxied to the Go backend (UMBRA_DEV=1 UMBRA_LISTEN=127.0.0.1:8967)
		proxy: { '/api': { target: 'http://127.0.0.1:8967', changeOrigin: false } }
	}
});
