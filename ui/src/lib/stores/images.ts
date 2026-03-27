import { writable } from 'svelte/store';
import type { Image } from '$lib/types';
import { api } from '$lib/services/api';

function createImagesStore() {
	const { subscribe, set } = writable<Image[]>([]);
	const loading = writable(false);
	const error = writable<string | null>(null);

	return {
		subscribe,
		loading: { subscribe: loading.subscribe },
		error: { subscribe: error.subscribe },

		async refresh() {
			loading.set(true);
			error.set(null);
			try {
				const data = await api.getImages();
				set(data.images);
			} catch (e) {
				error.set(e instanceof Error ? e.message : 'Failed to load images');
			} finally {
				loading.set(false);
			}
		}
	};
}

export const images = createImagesStore();
