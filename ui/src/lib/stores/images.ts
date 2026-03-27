import { writable } from 'svelte/store';
import type { Image } from '$lib/types';
import { api } from '$lib/services/api';

function createImagesStore() {
	const { subscribe, set } = writable<Image[]>([]);
	const loading = writable(false);
	const error = writable<string | null>(null);
	let cached = false;

	return {
		subscribe,
		loading: { subscribe: loading.subscribe },
		error: { subscribe: error.subscribe },

		async load(forceRefresh = false) {
			if (cached && !forceRefresh) {
				return;
			}

			loading.set(true);
			error.set(null);

			try {
				const data = await api.getImages();
				set(data.images);
				cached = true;
			} catch (e) {
				error.set(e instanceof Error ? e.message : 'Failed to load images');
			} finally {
				loading.set(false);
			}
		},

		async refresh() {
			await this.load(true);
		},

		get cached() {
			return cached;
		}
	};
}

export const images = createImagesStore();
