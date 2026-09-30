import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api, RateLimitedError } from '../src/lib/services/api';

function setCsrfToken(value: string | null): void {
	(api as unknown as { csrfToken: string | null }).csrfToken = value;
}

function uploadFile(): File {
	return new File(['data'], 'seed.txt', { type: 'text/plain' });
}

describe('ApiService uploadFile error handling', () => {
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

	it('throws RateLimitedError with the Retry-After delay on 429', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ error: 'rate_limited', message: 'Too many requests.' }), {
				status: 429,
				headers: { 'Retry-After': '30' }
			})
		);

		const err = await api.uploadFile('web-1', uploadFile()).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(RateLimitedError);
		expect((err as RateLimitedError).retryAfterMs).toBe(30000);
	});

	it('falls back to a default delay without a Retry-After header', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ error: 'rate_limited' }), { status: 429 })
		);

		const err = await api.uploadFile('web-1', uploadFile()).catch((e: unknown) => e);
		expect(err).toBeInstanceOf(RateLimitedError);
		expect((err as RateLimitedError).retryAfterMs).toBe(5000);
	});

	it('surfaces the backend message on JSON failures', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(
				JSON.stringify({ error: 'file_too_large', message: 'file size exceeds maximum' }),
				{
					status: 400,
					headers: { 'Content-Type': 'application/json' }
				}
			)
		);

		await expect(api.uploadFile('web-1', uploadFile())).rejects.toThrow(
			'file size exceeds maximum'
		);
	});

	it('does not throw a secondary error on empty/non-JSON failure bodies', async () => {
		fetchMock.mockResolvedValueOnce(new Response(null, { status: 500 }));

		await expect(api.uploadFile('web-1', uploadFile())).rejects.toThrow('Upload failed');
	});

	it('resolves with the upload response on success', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ message: 'File uploaded', path: '/home/ubuntu/seed.txt' }), {
				status: 201,
				headers: { 'Content-Type': 'application/json' }
			})
		);

		await expect(api.uploadFile('web-1', uploadFile(), '/home/ubuntu/')).resolves.toEqual({
			message: 'File uploaded',
			path: '/home/ubuntu/seed.txt'
		});

		const body = fetchMock.mock.calls[0][1].body as FormData;
		expect(body.get('target_path')).toBe('/home/ubuntu/');
		expect((body.get('file') as File).name).toBe('seed.txt');
	});
});
