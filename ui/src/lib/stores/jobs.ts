import { writable } from 'svelte/store';
import type { Job } from '$lib/types';
import { api } from '$lib/services/api';
import { instances } from './instances';
import { notifications } from './notifications';

interface JobEvent {
	type: string;
	job: Job;
}

function createJobsStore() {
	const { subscribe, set, update } = writable<Job[]>([]);
	const loading = writable(false);
	let pollInterval: ReturnType<typeof setInterval> | null = null;
	let eventSource: EventSource | null = null;

	async function checkAndRefresh() {
		try {
			const allJobs = await api.listJobs();
			const activeJobs = allJobs.filter(
				(job) => job.status === 'pending' || job.status === 'running'
			);

			const completedJobs = allJobs.filter(
				(job) => job.status === 'completed' || job.status === 'failed'
			);

			if (completedJobs.length > 0) {
				await instances.refresh();
			}

			set(activeJobs);
		} catch (e) {
			console.error('Failed to load jobs:', e);
		}
	}

	return {
		subscribe,
		loading: { subscribe: loading.subscribe },

		async refresh() {
			loading.set(true);
			try {
				await checkAndRefresh();
			} finally {
				loading.set(false);
			}
		},

		startPolling(intervalMs = 5000) {
			if (pollInterval) return;
			pollInterval = setInterval(checkAndRefresh, intervalMs);
		},

		stopPolling() {
			if (pollInterval) {
				clearInterval(pollInterval);
				pollInterval = null;
			}
		},

		startSSE() {
			this.stopPolling();

			if (typeof window === 'undefined') return;

			const apiUrl = import.meta.env.VITE_API_URL || '';
			const sseUrl = `${apiUrl}/api/jobs/stream`;

			try {
				eventSource = new EventSource(sseUrl);

				eventSource.onopen = () => {
					console.log('SSE connected');
				};

				eventSource.onmessage = async (event) => {
					try {
						const data: JobEvent = JSON.parse(event.data);
						if (data.job) {
							if (data.job.status === 'completed' || data.job.status === 'failed') {
								update((jobs) => jobs.filter((j) => j.id !== data.job.id));
								await instances.refresh();
								notifications.show(
									data.job.status === 'completed'
										? `Instance "${data.job.instance_name}" is ready`
										: `Instance "${data.job.instance_name}" failed: ${data.job.error || 'Unknown error'}`,
									data.job.status === 'completed' ? 'success' : 'error'
								);
							} else if (data.job.status === 'running' || data.job.status === 'pending') {
								update((jobs) => {
									const existing = jobs.find((j) => j.id === data.job.id);
									if (existing) {
										return jobs.map((j) => (j.id === data.job.id ? data.job : j));
									}
									return [...jobs, data.job];
								});
							}
						}
					} catch (e) {
						console.error('Failed to parse SSE message:', e);
					}
				};

				eventSource.onerror = () => {
					console.error('SSE connection lost, stopping stream');
					this.stopSSE();
				};
			} catch (e) {
				console.error('Failed to start SSE:', e);
			}
		},

		stopSSE() {
			if (eventSource) {
				eventSource.close();
				eventSource = null;
			}
		},

		remove(jobId: string) {
			update((jobs) => jobs.filter((j) => j.id !== jobId));
		}
	};
}

export const jobs = createJobsStore();
