import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { jobs } from '../src/lib/stores/jobs';
import { IPForbiddenError } from '../src/lib/services/api';

const { mockListJobs } = vi.hoisted(() => ({
	mockListJobs: vi.fn()
}));

vi.mock('$lib/services/api', async () => {
	const actual =
		await vi.importActual<typeof import('../src/lib/services/api')>('../src/lib/services/api');
	return {
		...actual,
		api: {
			listJobs: mockListJobs
		}
	};
});

class MockEventSource {
	static instances: MockEventSource[] = [];
	onopen: ((ev: Event) => void) | null = null;
	onmessage: ((ev: MessageEvent) => void) | null = null;
	onerror: (() => void) | null = null;
	close = vi.fn();
	url: string;

	constructor(url: string) {
		this.url = url;
		MockEventSource.instances.push(this);
	}

	triggerError(): void {
		this.onerror?.();
	}
}

describe('jobs SSE fallback', () => {
	beforeEach(() => {
		vi.stubGlobal('EventSource', MockEventSource);
		vi.useFakeTimers();
		MockEventSource.instances = [];
		mockListJobs.mockReset();
	});

	afterEach(() => {
		jobs.stopSSE();
		jobs.stopPolling();
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	function latestStream(): MockEventSource {
		const stream = MockEventSource.instances[MockEventSource.instances.length - 1];
		if (!stream) throw new Error('no EventSource created');
		return stream;
	}

	it('falls back to polling when the stream fails but the backend is reachable', async () => {
		mockListJobs.mockResolvedValue([]);
		jobs.startSSE();

		latestStream().triggerError();
		await vi.advanceTimersByTimeAsync(0);
		expect(mockListJobs).toHaveBeenCalledTimes(1);

		await vi.advanceTimersByTimeAsync(5000);
		expect(mockListJobs).toHaveBeenCalledTimes(2);
	});

	it('does not poll after an IP denial (redirect already happened)', async () => {
		mockListJobs.mockRejectedValue(new IPForbiddenError('access denied'));
		jobs.startSSE();

		latestStream().triggerError();
		await vi.advanceTimersByTimeAsync(0);
		expect(mockListJobs).toHaveBeenCalledTimes(1);

		await vi.advanceTimersByTimeAsync(15000);
		expect(mockListJobs).toHaveBeenCalledTimes(1);
	});

	it('keeps polling through transient backend outages', async () => {
		mockListJobs.mockRejectedValueOnce(new Error('connection refused'));
		mockListJobs.mockResolvedValue([]);
		jobs.startSSE();

		latestStream().triggerError();
		await vi.advanceTimersByTimeAsync(0);
		expect(mockListJobs).toHaveBeenCalledTimes(1);

		await vi.advanceTimersByTimeAsync(5000);
		expect(mockListJobs).toHaveBeenCalledTimes(2);
	});
});
