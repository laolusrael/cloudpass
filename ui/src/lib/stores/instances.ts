import { writable, derived } from 'svelte/store';
import type { Instance } from '$lib/types';
import { api, RateLimitedError } from '$lib/services/api';

const STORED_NOTIFICATION_KEY = 'cloudpass_pending_notifications';
const MAX_JOB_POLL_ATTEMPTS = 180; // 180 * 2s = 6 minutes timeout
const JOB_POLL_INTERVAL_MS = 2000;
const MAX_POST_RETRIES = 5;
const MAX_POLL_BACKOFFS = 10;
const MAX_BACKOFF_WAIT_MS = 120000;

/** Thrown when a create is superseded or cancelled. Callers should swallow it. */
export class CreateCancelledError extends Error {
	constructor() {
		super('Instance creation cancelled');
		this.name = 'CreateCancelledError';
	}
}

function isAbort(e: unknown, signal: AbortSignal): boolean {
	return (
		signal.aborted ||
		(typeof DOMException !== 'undefined' && e instanceof DOMException && e.name === 'AbortError')
	);
}

function abortableSleep(ms: number, signal: AbortSignal): Promise<void> {
	if (signal.aborted) {
		return Promise.reject(new CreateCancelledError());
	}
	return new Promise<void>((resolve, reject) => {
		const timer = setTimeout(() => {
			signal.removeEventListener('abort', onAbort);
			resolve();
		}, ms);
		const onAbort = () => {
			clearTimeout(timer);
			reject(new CreateCancelledError());
		};
		signal.addEventListener('abort', onAbort, { once: true });
	});
}

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

	// At most one create flow runs at a time: a new create supersedes any
	// running one, so retries and navigations can never stack pollers and
	// exhaust the per-IP rate budget (the 429 spiral).
	let activeCreateAbort: AbortController | null = null;

	function cancelCreate() {
		activeCreateAbort?.abort();
		activeCreateAbort = null;
	}

	async function postJobWithRetry(
		request: Parameters<typeof api.createInstance>[0],
		idempotencyKey: string,
		signal: AbortSignal,
		onProgress?: (status: string) => void
	) {
		let retries = 0;
		for (;;) {
			try {
				return await api.createInstanceAsync(request, { idempotencyKey, signal });
			} catch (e) {
				if (isAbort(e, signal)) {
					throw new CreateCancelledError();
				}
				if (e instanceof RateLimitedError && retries < MAX_POST_RETRIES) {
					retries++;
					if (onProgress) {
						onProgress('Server is busy, retrying…');
					}
					await abortableSleep(Math.min(e.retryAfterMs, MAX_BACKOFF_WAIT_MS), signal);
					continue;
				}
				throw e;
			}
		}
	}

	async function pollJob(
		jobId: string,
		request: Parameters<typeof api.createInstance>[0],
		signal: AbortSignal,
		onProgress?: (status: string) => void
	): Promise<Instance> {
		let attempts = 0;
		let backoffs = 0;
		for (;;) {
			await abortableSleep(JOB_POLL_INTERVAL_MS, signal);

			let updatedJob;
			try {
				updatedJob = await api.getJob(jobId, signal);
			} catch (e) {
				if (isAbort(e, signal)) {
					throw new CreateCancelledError();
				}
				if (e instanceof RateLimitedError) {
					backoffs++;
					if (backoffs > MAX_POLL_BACKOFFS) {
						throw new Error('Server is busy. Please try again later.');
					}
					if (onProgress) {
						onProgress('Server is busy, retrying…');
					}
					await abortableSleep(Math.min(e.retryAfterMs, MAX_BACKOFF_WAIT_MS), signal);
					continue;
				}
				throw e;
			}
			attempts++;

			if (onProgress) {
				onProgress(updatedJob.status);
			}

			if (updatedJob.status === 'completed') {
				const instance = await api.getInstance(updatedJob.instance_name!, signal);
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

			if (attempts >= MAX_JOB_POLL_ATTEMPTS) {
				throw new Error(
					'Instance creation timed out after ' +
						(MAX_JOB_POLL_ATTEMPTS * JOB_POLL_INTERVAL_MS) / 1000 +
						' seconds'
				);
			}
		}
	}

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

		cancelCreate,

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
			// Single-flight: supersede any running create before starting.
			cancelCreate();
			const controller = new AbortController();
			activeCreateAbort = controller;
			const signal = controller.signal;
			// One key per user-initiated attempt: automatic retries of this
			// attempt are safe (server dedupes); a deliberate user retry
			// generates a fresh key via a fresh createAsync call.
			const idempotencyKey = crypto.randomUUID();

			loading.set(true);
			error.set(null);

			try {
				const job = await postJobWithRetry(request, idempotencyKey, signal, onProgress);
				return await pollJob(job.id, request, signal, onProgress);
			} catch (e) {
				if (isAbort(e, signal)) {
					throw new CreateCancelledError();
				}
				error.set(e instanceof Error ? e.message : 'Failed to create instance');
				throw e;
			} finally {
				if (activeCreateAbort === controller) {
					activeCreateAbort = null;
				}
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
