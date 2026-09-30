import { api } from '$lib/services/api';

/** Rejection reason when a job does not settle in time. */
export class JobTimeoutError extends Error {
	jobId: string;

	constructor(jobId: string, timeoutMs: number) {
		super(`Job ${jobId} did not complete within ${Math.round(timeoutMs / 1000)}s`);
		this.name = 'JobTimeoutError';
		this.jobId = jobId;
	}
}

export interface WaitForJobOptions {
	intervalMs?: number;
	timeoutMs?: number;
	onPending?: (status: string) => void;
}

/**
 * Poll a background job until it completes or fails. Resolves with the
 * job result message; rejects with the job error or JobTimeoutError.
 * Pages use this for their own completion UX; the jobs store separately
 * broadcasts global notifications for create/import jobs.
 */
export async function waitForJob(jobId: string, options: WaitForJobOptions = {}): Promise<string> {
	const intervalMs = options.intervalMs ?? 3000;
	const timeoutMs = options.timeoutMs ?? 10 * 60 * 1000;
	const deadline = Date.now() + timeoutMs;

	for (;;) {
		const job = await api.getJob(jobId);
		if (job.status === 'completed') {
			return job.result ?? '';
		}
		if (job.status === 'failed') {
			throw new Error(job.error || 'Operation failed');
		}
		options.onPending?.(job.status);
		if (Date.now() >= deadline) {
			throw new JobTimeoutError(jobId, timeoutMs);
		}
		await new Promise((resolve) => setTimeout(resolve, intervalMs));
	}
}
