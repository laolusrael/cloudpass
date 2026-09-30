import { describe, it, expect, vi, beforeEach } from 'vitest';
import { waitForJob, JobTimeoutError } from '../src/lib/utils/job-poller';
import type { Job } from '../src/lib/types/instance';

const { mockGetJob } = vi.hoisted(() => ({
	mockGetJob: vi.fn()
}));

vi.mock('$lib/services/api', () => ({
	api: { getJob: mockGetJob }
}));

function job(status: Job['status'], extra: Partial<Job> = {}): Job {
	return {
		id: 'job-1',
		type: 'mount',
		status,
		created_at: '',
		updated_at: '',
		...extra
	};
}

describe('waitForJob', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('resolves with the result on completion', async () => {
		mockGetJob
			.mockResolvedValueOnce(job('pending'))
			.mockResolvedValueOnce(job('running'))
			.mockResolvedValueOnce(job('completed', { result: '/a -> /b' }));

		await expect(waitForJob('job-1', { intervalMs: 1 })).resolves.toBe('/a -> /b');
		expect(mockGetJob).toHaveBeenCalledTimes(3);
	});

	it('rejects with the job error on failure', async () => {
		mockGetJob.mockResolvedValueOnce(job('failed', { error: 'mount exploded' }));

		await expect(waitForJob('job-1', { intervalMs: 1 })).rejects.toThrow('mount exploded');
	});

	it('rejects with JobTimeoutError past the deadline', async () => {
		mockGetJob.mockResolvedValue(job('running'));

		await expect(waitForJob('job-1', { intervalMs: 1, timeoutMs: 5 })).rejects.toThrow(
			JobTimeoutError
		);
	});

	it('reports pending statuses', async () => {
		const seen: string[] = [];
		mockGetJob
			.mockResolvedValueOnce(job('pending'))
			.mockResolvedValueOnce(job('completed', { result: 'done' }));

		await waitForJob('job-1', { intervalMs: 1, onPending: (s) => seen.push(s) });
		expect(seen).toEqual(['pending']);
	});
});
