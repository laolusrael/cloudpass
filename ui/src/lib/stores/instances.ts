import { writable, derived } from 'svelte/store';
import type { Instance } from '$lib/types';
import { api } from '$lib/services/api';

const STORED_NOTIFICATION_KEY = 'cloudpass_pending_notifications';
const MAX_JOB_POLL_ATTEMPTS = 180; // 180 * 2s = 6 minutes timeout
const JOB_POLL_INTERVAL_MS = 2000;

export interface StoredNotification {
	instanceName: string;
	status: 'completed' | 'failed';
	message: string;
	timestamp: number;
}

function createInstancesStore() {
	const { subscribe, set, update } = writable<Instance[]>([]);
	const loading = writable(false);
	const error = writable<string | null>(null);

	const refresh = async () => {
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
	};

	return {
		subscribe,
		loading: { subscribe: loading.subscribe },
		error: { subscribe: error.subscribe },

		refresh,

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

		async createAsync(
			request: Parameters<typeof api.createInstance>[0],
			onProgress?: (status: string) => void
		): Promise<Instance> {
			loading.set(true);
			error.set(null);

			try {
				const job = await api.createInstanceAsync(request);

				const pollJob = async (): Promise<Instance> => {
					for (let attempt = 0; attempt < MAX_JOB_POLL_ATTEMPTS; attempt++) {
						await new Promise((resolve) => setTimeout(resolve, JOB_POLL_INTERVAL_MS));

						const updatedJob = await api.getJob(job.id);

						if (onProgress) {
							onProgress(updatedJob.status);
						}

						if (updatedJob.status === 'completed') {
							const instance = await api.getInstance(updatedJob.instance_name!);
							update((instances) => [...instances, instance]);

							const notification: StoredNotification = {
								instanceName: instance.name,
								status: 'completed',
								message: `Instance "${instance.name}" created successfully`,
								timestamp: Date.now()
							};
							savePendingNotification(notification);

							return instance;
						}

						if (updatedJob.status === 'failed') {
							const notification: StoredNotification = {
								instanceName: request.name || 'unknown',
								status: 'failed',
								message: updatedJob.error || 'Instance creation failed',
								timestamp: Date.now()
							};
							savePendingNotification(notification);

							throw new Error(updatedJob.error || 'Instance creation failed');
						}
					}

					throw new Error('Instance creation timed out after ' + (MAX_JOB_POLL_ATTEMPTS * JOB_POLL_INTERVAL_MS / 1000) + ' seconds');
				};

				return await pollJob();
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
			await refresh();
		},

		async stop(name: string) {
			await api.stopInstance(name);
			await refresh();
		},

		async restart(name: string) {
			await api.restartInstance(name);
			await refresh();
		}
	};
}

function savePendingNotification(notification: StoredNotification) {
	try {
		const existing = getStoredNotifications();
		existing.push(notification);
		localStorage.setItem(STORED_NOTIFICATION_KEY, JSON.stringify(existing));
	} catch (e) {
		console.error('Failed to save notification:', e);
	}
}

function getStoredNotifications(): StoredNotification[] {
	try {
		const stored = localStorage.getItem(STORED_NOTIFICATION_KEY);
		return stored ? JSON.parse(stored) : [];
	} catch {
		return [];
	}
}

export function clearStoredNotification(timestamp: number) {
	try {
		const existing = getStoredNotifications().filter((n) => n.timestamp !== timestamp);
		localStorage.setItem(STORED_NOTIFICATION_KEY, JSON.stringify(existing));
	} catch (e) {
		console.error('Failed to clear notification:', e);
	}
}

export function getAndClearStoredNotifications(): StoredNotification[] {
	const notifications = getStoredNotifications();
	localStorage.removeItem(STORED_NOTIFICATION_KEY);
	return notifications;
}

export const instances = createInstancesStore();

export const runningInstances = derived(instances, ($instances) =>
	$instances.filter((i) => i.state === 'Running')
);

export const stoppedInstances = derived(instances, ($instances) =>
	$instances.filter((i) => i.state === 'Stopped')
);
