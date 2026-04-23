import { defineConfig, mergeConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import viteConfig from './vite.config';

export default mergeConfig(
	viteConfig,
	defineConfig({
		plugins: [svelte({ hot: !process.env.VITEST })],
		test: {
			include: ['tests/**/*.test.ts'],
			exclude: ['tests/e2e/**'],
			environment: 'jsdom'
		}
	})
);
