import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import { instances, CreateCancelledError } from '../src/lib/stores/instances';
import { RateLimitedError } from '../src/lib/services/api';
import type { Job } from '../src/lib/types/instance';

const { mockCreateAsync, mockGetJob, mockGetInstance, mockGetInstances } = vi.hoisted(() => ({
	mockCreateAsync: vi.fn(),
	mockGetJob: vi.fn(),
	mockGetInstance: vi.fn(),
	mockGetInstances: vi.fn()
}));

vi.mock('$lib/services/api', async () => {
	const actual =
		await vi.importActual<typeof import('../src/lib/services/api')>('../src/lib/services/api');
	return {
		...actual,
		api: {
			createInstanceAsync: mockCreateAsync,
			getJob: mockGetJob,
			getInstance: mockGetInstance,
			getInstances: mockGetInstances
		}
	};
});

function pendingJob(id: string): Job {
	return { id, status: 'pending', instance_name: 'web-1' } as Job;
}

function completedJob(id: string): Job {
	return { id, status: 'completed', instance_name: 'web-1' } as Job;
}

describe('instances createAsync', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.clearAllMocks();
		mockGetInstances.mockResolvedValue([]);
	});

	afterEach(() => {
		instances.cancelCreate();
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('supersedes a running create so only one poller survives', async () => {
		mockCreateAsync
			.mockResolvedValueOnce(pendingJob('job-1'))
			.mockResolvedValueOnce(pendingJob('job-2'));
		mockGetJob.mockImplementation((id: string) => Promise.resolve(completedJob(id)));
		mockGetInstance.mockImplementation((name: string) =>
			Promise.resolve({ name, state: 'Running' })
		);

		const first = instances.createAsync({ name: 'web-1' });
		const second = instances.createAsync({ name: 'web-2' });

		await expect(first).rejects.toThrow(CreateCancelledError);
		await vi.advanceTimersByTimeAsync(3000);
		const instance = await second;

		expect(instance.name).toBe('web-1');
		const polledIds = mockGetJob.mock.calls.map((call) => (call as string[])[0]);
		expect(polledIds.length).toBeGreaterThan(0);
		expect(new Set(polledIds)).toEqual(new Set(['job-2']));
	});

	it('stops polling once cancelled', async () => {
		mockCreateAsync.mockResolvedValue(pendingJob('job-1'));
		mockGetJob.mockImplementation((id: string) => Promise.resolve(pendingJob(id)));

		const pending = instances.createAsync({ name: 'web-1' });
		// Flush the initial POST, then let one poll happen.
		await vi.advanceTimersByTimeAsync(2500);
		expect(mockGetJob).toHaveBeenCalled();

		const callsBefore = mockGetJob.mock.calls.length;
		instances.cancelCreate();
		await expect(pending).rejects.toThrow(CreateCancelledError);
		await vi.advanceTimersByTimeAsync(30000);
		expect(mockGetJob.mock.calls.length).toBe(callsBefore);
	});

	it('backs off on poll 429s and then resumes', async () => {
		mockCreateAsync.mockResolvedValue(pendingJob('job-1'));
		mockGetJob
			.mockRejectedValueOnce(new RateLimitedError(2000))
			.mockImplementation((id: string) => Promise.resolve(completedJob(id)));
		mockGetInstance.mockImplementation((name: string) =>
			Promise.resolve({ name, state: 'Running' })
		);
		const statuses: string[] = [];

		const done = instances.createAsync({ name: 'web-1' }, (s) => statuses.push(s));
		await vi.advanceTimersByTimeAsync(20000);
		await expect(done).resolves.toMatchObject({ name: 'web-1' });

		expect(statuses).toContain('Server is busy, retrying…');
		expect(get(instances)).toContainEqual({ name: 'web-1', state: 'Running' });
	});

	it('retries a rate-limited POST with the same idempotency key', async () => {
		mockCreateAsync
			.mockRejectedValueOnce(new RateLimitedError(1000))
			.mockResolvedValue(pendingJob('job-9'));
		mockGetJob.mockImplementation((id: string) => Promise.resolve(completedJob(id)));
		mockGetInstance.mockImplementation((name: string) =>
			Promise.resolve({ name, state: 'Running' })
		);

		const done = instances.createAsync({ name: 'web-1' });
		await vi.advanceTimersByTimeAsync(20000);
		await expect(done).resolves.toMatchObject({ name: 'web-1' });

		expect(mockCreateAsync).toHaveBeenCalledTimes(2);
		const firstKey = (mockCreateAsync.mock.calls[0] as unknown[])[1] as {
			idempotencyKey?: string;
		};
		const secondKey = (mockCreateAsync.mock.calls[1] as unknown[])[1] as {
			idempotencyKey?: string;
		};
		expect(firstKey.idempotencyKey).toMatch(/^[0-9a-f-]{36}$/);
		expect(secondKey.idempotencyKey).toBe(firstKey.idempotencyKey);
	});

	it('gives up after repeated poll backoffs', async () => {
		mockCreateAsync.mockResolvedValue(pendingJob('job-1'));
		mockGetJob.mockRejectedValue(new RateLimitedError(1000));

		const done = instances.createAsync({ name: 'web-1' });
		// Attach before advancing: the rejection fires inside timer
		// callbacks, otherwise it reads as unhandled.
		const assertion = expect(done).rejects.toThrow('Server is busy');
		await vi.advanceTimersByTimeAsync(600000);
		await assertion;
	});
});
