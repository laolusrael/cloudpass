import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api, RateLimitedError } from '../src/lib/services/api';

type FetchArgs = [string, RequestInit | undefined];

function setCsrfToken(value: string | null): void {
	(api as unknown as { csrfToken: string | null }).csrfToken = value;
}

describe('ApiService rate limiting', () => {
	const fetchMock = vi.fn();

	beforeEach(() => {
		vi.stubGlobal('fetch', fetchMock);
		fetchMock.mockReset();
		setCsrfToken(null);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		setCsrfToken(null);
	});

	it('throws RateLimitedError with the Retry-After delay', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ error: 'rate_limited', message: 'Too many requests.' }), {
				status: 429,
				headers: { 'Retry-After': '60' }
			})
		);

		const err = await api.getInstances().catch((e: unknown) => e);
		expect(err).toBeInstanceOf(RateLimitedError);
		expect((err as RateLimitedError).retryAfterMs).toBe(60000);
	});

	it('falls back to a default delay without a Retry-After header', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ error: 'rate_limited' }), { status: 429 })
		);

		const err = await api.getInstances().catch((e: unknown) => e);
		expect(err).toBeInstanceOf(RateLimitedError);
		expect((err as RateLimitedError).retryAfterMs).toBe(5000);
	});

	it('sends the idempotency key on async creates', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ job: { id: 'job-1', status: 'pending' } }), {
				status: 202
			})
		);

		await api.createInstanceAsync({ name: 'web-1' }, { idempotencyKey: 'key-abc-123' });

		const calls = fetchMock.mock.calls as FetchArgs[];
		expect(calls).toHaveLength(1);
		const headers = (calls[0][1] && calls[0][1].headers) as Record<string, string>;
		expect(headers['Idempotency-Key']).toBe('key-abc-123');
	});

	it('omits the header when no key is given (legacy behavior)', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ job: { id: 'job-1', status: 'pending' } }), {
				status: 202
			})
		);

		await api.createInstanceAsync({ name: 'web-1' });

		const calls = fetchMock.mock.calls as FetchArgs[];
		const headers = (calls[0][1] && calls[0][1].headers) as Record<string, string>;
		expect(headers['Idempotency-Key']).toBeUndefined();
	});
});
