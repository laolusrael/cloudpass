import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api, IPForbiddenError, CSRFExpiredError } from '../src/lib/services/api';

type FetchArgs = [string, RequestInit | undefined];

function jsonResponse(body: unknown, status: number): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});
}

function setCsrfToken(value: string | null): void {
	(api as unknown as { csrfToken: string | null }).csrfToken = value;
}

function fetchCalls(fetchMock: ReturnType<typeof vi.fn>): FetchArgs[] {
	return fetchMock.mock.calls as FetchArgs[];
}

function requestHeaders(call: FetchArgs): Record<string, string> {
	return ((call[1] && call[1].headers) || {}) as Record<string, string>;
}

describe('ApiService 403 handling', () => {
	const fetchMock = vi.fn();
	const originalLocationDescriptor = Object.getOwnPropertyDescriptor(window, 'location');
	let redirectTarget: { href: string };

	beforeEach(() => {
		vi.stubGlobal('fetch', fetchMock);
		fetchMock.mockReset();
		redirectTarget = { href: '' };
		Object.defineProperty(window, 'location', {
			value: redirectTarget,
			writable: true,
			configurable: true
		});
		setCsrfToken(null);
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		if (originalLocationDescriptor) {
			Object.defineProperty(window, 'location', originalLocationDescriptor);
		}
	});

	it('redirects to /unauthorized on IP denials (403 forbidden)', async () => {
		fetchMock.mockResolvedValue(
			jsonResponse({ error: 'forbidden', message: 'access denied: IP not whitelisted' }, 403)
		);

		const err = await api.getInstances().catch((e: unknown) => e);
		expect(err).toBeInstanceOf(IPForbiddenError);
		expect((err as Error).message).toBe('access denied: IP not whitelisted');
		expect(redirectTarget.href).toBe('/unauthorized');
	});

	it('retries once with a fresh token on CSRF expiry, then succeeds', async () => {
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({ error: 'csrf_session_expired', message: 'Session has expired.' }, 403)
			)
			.mockResolvedValueOnce(jsonResponse({ csrf_token: 'tok123' }, 200))
			.mockResolvedValueOnce(jsonResponse({ instances: [] }, 200));

		const instance = await api.createInstance({ name: 'web-1' });

		expect(instance).toEqual({ instances: [] });
		const calls = fetchCalls(fetchMock);
		expect(calls).toHaveLength(3);
		expect(calls[1][0]).toBe('/api/csrf/token');
		expect(requestHeaders(calls[2])['X-CSRF-Token']).toBe('tok123');
		expect(redirectTarget.href).toBe('');
	});

	it('surfaces CSRF errors without redirect when the retry also fails', async () => {
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({ error: 'csrf_token_invalid', message: 'CSRF token is invalid' }, 403)
			)
			.mockResolvedValueOnce(jsonResponse({ csrf_token: 'tok456' }, 200))
			.mockResolvedValueOnce(
				jsonResponse({ error: 'csrf_token_invalid', message: 'CSRF token is invalid' }, 403)
			);

		await expect(api.createInstance({ name: 'web-1' })).rejects.toThrow(CSRFExpiredError);
		expect(fetchMock).toHaveBeenCalledTimes(3);
		expect(redirectTarget.href).toBe('');
	});

	it('does not retry safe-method requests on CSRF errors', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse({ error: 'csrf_token_missing', message: 'CSRF token is required' }, 403)
		);

		await expect(api.getInstances()).rejects.toThrow(CSRFExpiredError);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		expect(redirectTarget.href).toBe('');
	});

	it('handles empty/non-JSON 403 bodies without redirect', async () => {
		fetchMock.mockResolvedValueOnce(new Response(null, { status: 403 }));

		await expect(api.getInstances()).rejects.toThrow('Access denied');
		expect(redirectTarget.href).toBe('');
	});

	it('still surfaces non-403 backend errors unchanged', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse({ error: 'invalid_request', message: 'instance name is required' }, 400)
		);

		await expect(api.getInstances()).rejects.toThrow('instance name is required');
		expect(redirectTarget.href).toBe('');
	});

	it('returns parsed bodies on success', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({ instances: [{ name: 'web-1' }] }, 200));

		await expect(api.getInstances()).resolves.toEqual([{ name: 'web-1' }]);
	});

	it('uploadFile sends credentials and CSRF token, and redirects on IP denial', async () => {
		setCsrfToken('up-tok');
		fetchMock.mockResolvedValueOnce(
			jsonResponse({ error: 'forbidden', message: 'access denied: IP not whitelisted' }, 403)
		);

		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		await expect(api.uploadFile('web-1', file)).rejects.toThrow(IPForbiddenError);

		const calls = fetchCalls(fetchMock);
		expect(calls).toHaveLength(1);
		expect(calls[0][1] && calls[0][1].credentials).toBe('include');
		expect(requestHeaders(calls[0])['X-CSRF-Token']).toBe('up-tok');
		expect(redirectTarget.href).toBe('/unauthorized');
	});

	it('shares one CSRF refresh across concurrent mutating requests', async () => {
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({ error: 'csrf_token_invalid', message: 'CSRF token is invalid' }, 403)
			)
			.mockResolvedValueOnce(
				jsonResponse({ error: 'csrf_token_invalid', message: 'CSRF token is invalid' }, 403)
			)
			.mockResolvedValueOnce(jsonResponse({ csrf_token: 'shared-tok' }, 200))
			.mockResolvedValueOnce(jsonResponse({ name: 'a' }, 200))
			.mockResolvedValueOnce(jsonResponse({ name: 'b' }, 200));

		const [a, b] = await Promise.all([
			api.createInstance({ name: 'a' }),
			api.createInstance({ name: 'b' })
		]);

		expect(a).toEqual({ name: 'a' });
		expect(b).toEqual({ name: 'b' });
		const tokenFetches = fetchCalls(fetchMock).filter((call) => call[0] === '/api/csrf/token');
		expect(tokenFetches).toHaveLength(1);
		expect(redirectTarget.href).toBe('');
	});

	it('uploadFile retries once after refreshing an expired token', async () => {
		fetchMock
			.mockResolvedValueOnce(
				jsonResponse({ error: 'csrf_token_missing', message: 'CSRF token is required' }, 403)
			)
			.mockResolvedValueOnce(jsonResponse({ csrf_token: 'up-tok-2' }, 200))
			.mockResolvedValueOnce(jsonResponse({ message: 'File uploaded', path: '/x' }, 201));

		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		await expect(api.uploadFile('web-1', file)).resolves.toEqual({
			message: 'File uploaded',
			path: '/x'
		});

		const calls = fetchCalls(fetchMock);
		expect(calls).toHaveLength(3);
		expect(requestHeaders(calls[2])['X-CSRF-Token']).toBe('up-tok-2');
		expect(redirectTarget.href).toBe('');
	});
});
