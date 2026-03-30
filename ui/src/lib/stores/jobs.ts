import { writable } from 'svelte/store';
import type { Job } from '$lib/types';
import { api } from '$lib/services/api';
import { instances } from './instances';

function createJobsStore() {
	const { subscribe, set, update } = writable<Job[]>([]);
	const loading = writable(false);
	let pollInterval: ReturnType<typeof setInterval> | null = null;

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

		remove(jobId: string) {
			update((jobs) => jobs.filter((j) => j.id !== jobId));
		}
	};
}

export const jobs = createJobsStore();
