import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api } from '../src/lib/services/api';

function setCsrfToken(value: string | null): void {
	(api as unknown as { csrfToken: string | null }).csrfToken = value;
}

describe('ApiService request timeout', () => {
	const fetchMock = vi.fn();

	beforeEach(() => {
		vi.stubGlobal('fetch', fetchMock);
		fetchMock.mockReset();
		setCsrfToken(null);
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
		setCsrfToken(null);
	});

	it('rejects with a timeout error when the server never responds', async () => {
		// Behaves like real fetch: rejects once the signal aborts.
		fetchMock.mockImplementationOnce(
			(_url: string, init: RequestInit) =>
				new Promise((_resolve, reject) => {
					init.signal?.addEventListener('abort', () => {
						reject(new DOMException('Aborted', 'AbortError'));
					});
				})
		);

		const pending = api.getInstances();
		const assertion = expect(pending).rejects.toThrow('Request timed out');
		await vi.advanceTimersByTimeAsync(60000);
		await assertion;
	});

	it('does not time out fast responses', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ instances: [] }), { status: 200 })
		);

		await expect(api.getInstances()).resolves.toEqual([]);
	});

	it('leaves caller-owned signals alone', async () => {
		const controller = new AbortController();
		let settled = false;
		fetchMock.mockReturnValueOnce(
			new Promise((_, reject) => {
				controller.signal.addEventListener('abort', () => {
					settled = true;
					reject(new DOMException('Aborted', 'AbortError'));
				});
			})
		);

		const pending = api.createInstanceAsync({ name: 'web-1' }, { signal: controller.signal });
		// Advance well past the default budget: the client must not arm its
		// own timer when the caller owns the signal.
		await vi.advanceTimersByTimeAsync(120000);
		expect(settled).toBe(false);

		controller.abort();
		await expect(pending).rejects.toThrow('Aborted');
		expect(settled).toBe(true);
	});
});
