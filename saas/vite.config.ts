import { svelte } from '@sveltejs/vite-plugin-svelte';
import tailwindcss from '@tailwindcss/vite';
import path from 'node:path';
import { defineConfig } from 'vite';

const apiTarget = process.env.VITE_API_TARGET || 'http://localhost:7819';

export default defineConfig({
	plugins: [tailwindcss(), svelte()],
	resolve: {
		alias: {
			$lib: path.resolve('./src/lib')
		}
	},
	server: {
		host: true,
		port: 5173,
		proxy: {
			'/api': {
				target: apiTarget,
				changeOrigin: true
			}
		}
	}
});
