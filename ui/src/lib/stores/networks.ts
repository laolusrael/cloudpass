import { writable } from 'svelte/store';
import type { Network } from '$lib/types';

function createNetworksStore() {
	const { subscribe, set, update } = writable<Network[]>([]);

	return {
		subscribe,
		set,
		async refresh() {
			const { api } = await import('$lib/services/api');
			const data = await api.getNetworks();
			set(data.networks);
		},
		add(network: Network) {
			update(networks => [...networks, network]);
		},
		remove(name: string) {
			update(networks => networks.filter(n => n.name !== name));
		}
	};
}

export const networks = createNetworksStore();
