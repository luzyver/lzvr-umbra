import { defineConfig } from 'vite';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import adapter from '@sveltejs/adapter-static';

// Hasil build masuk ke internal/ui/dist lalu di-embed ke binary Go.
const out = '../internal/ui/dist';

export default defineConfig({
	plugins: [
		tailwindcss(),
		sveltekit({
			adapter: adapter({ pages: out, assets: out, fallback: 'index.html' })
		})
	],
	server: {
		// npm run dev: API diteruskan ke backend Go (UMBRA_DEV=1 UMBRA_LISTEN=127.0.0.1:8080)
		proxy: { '/api': { target: 'http://127.0.0.1:8080', changeOrigin: false } }
	}
});
