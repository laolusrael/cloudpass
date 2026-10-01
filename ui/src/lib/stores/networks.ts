import { writable } from 'svelte/store';
import type { Network } from '$lib/types';

function createNetworksStore() {
	const { subscribe, set, update } = writable<Network[]>([]);
	const unverified = writable<string[]>([]);
	const loading = writable(false);
	const error = writable<string | null>(null);

	async function refresh() {
		loading.set(true);
		error.set(null);
		try {
			const { api } = await import('$lib/services/api');
			const data = await api.getNetworks();
			set(data.networks ?? []);
			unverified.set(data.unverified_instances ?? []);
		} catch (e) {
			error.set(e instanceof Error ? e.message : 'Failed to load networks');
		} finally {
			loading.set(false);
		}
	}

	return {
		subscribe,
		set,
		unverified: { subscribe: unverified.subscribe },
		loading: { subscribe: loading.subscribe },
		error: { subscribe: error.subscribe },

		refresh,

		add(network: Network) {
			update((networks) => [...networks, network]);
		},
		remove(name: string) {
			update((networks) => networks.filter((n) => n.name !== name));
		}
	};
}

export const networks = createNetworksStore();
