import { writable, derived } from 'svelte/store';
import type { Instance } from '$lib/types';
import { api } from '$lib/services/api';

function createInstancesStore() {
	const { subscribe, set, update } = writable<Instance[]>([]);
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
				const instances = await api.getInstances();
				set(instances);
			} catch (e) {
				error.set(e instanceof Error ? e.message : 'Failed to load instances');
			} finally {
				loading.set(false);
			}
		},

		async create(request: Parameters<typeof api.createInstance>[0]) {
			loading.set(true);
			error.set(null);
			try {
				const instance = await api.createInstance(request);
				update((instances) => [...instances, instance]);
				return instance;
			} catch (e) {
				error.set(e instanceof Error ? e.message : 'Failed to create instance');
				throw e;
			} finally {
				loading.set(false);
			}
		},

		async delete(name: string) {
			loading.set(true);
			error.set(null);
			try {
				await api.deleteInstance(name);
				update((instances) => instances.filter((i) => i.name !== name));
			} catch (e) {
				error.set(e instanceof Error ? e.message : 'Failed to delete instance');
				throw e;
			} finally {
				loading.set(false);
			}
		},

		async start(name: string) {
			await api.startInstance(name);
			await this.refresh();
		},

		async stop(name: string) {
			await api.stopInstance(name);
			await this.refresh();
		},

		async restart(name: string) {
			await api.restartInstance(name);
			await this.refresh();
		}
	};
}

export const instances = createInstancesStore();

export const runningInstances = derived(instances, ($instances) =>
	$instances.filter((i) => i.state === 'Running')
);

export const stoppedInstances = derived(instances, ($instances) =>
	$instances.filter((i) => i.state === 'Stopped')
);
