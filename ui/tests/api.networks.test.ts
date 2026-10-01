import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api, NetworkInUseError } from '../src/lib/services/api';

type FetchArgs = [string, RequestInit | undefined];

function setCsrfToken(value: string | null): void {
	(api as unknown as { csrfToken: string | null }).csrfToken = value;
}

function jsonResponse(body: unknown, status: number): Response {
	return new Response(JSON.stringify(body), { status });
}

describe('ApiService networks', () => {
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

	it('throws NetworkInUseError with the blocking instances on 409', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse(
				{
					error: 'network_in_use',
					message: 'network "br01" is in use by: web-1',
					used_by: ['web-1']
				},
				409
			)
		);

		const err = await api.deleteNetwork('br01').catch((e: unknown) => e);
		expect(err).toBeInstanceOf(NetworkInUseError);
		expect((err as NetworkInUseError).usedBy).toEqual(['web-1']);
		expect((err as Error).message).toContain('web-1');
	});

	it('claims and unclaims networks', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({ message: 'claimed' }, 200));
		await api.claimNetwork('br01');
		fetchMock.mockResolvedValueOnce(jsonResponse({ message: 'unclaimed' }, 200));
		await api.unclaimNetwork('br01');

		const calls = fetchMock.mock.calls as FetchArgs[];
		expect(calls).toHaveLength(2);
		expect(calls[0][0]).toBe('/api/networks/br01/claim');
		expect((calls[0][1] && calls[0][1].method) || 'GET').toBe('POST');
		expect(calls[1][0]).toBe('/api/networks/br01/claim');
		expect(calls[1][1] && calls[1][1].method).toBe('DELETE');
	});

	it('gets and sets the instance network attribution', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse({ name: 'web-1', network: 'br01', verified: true }, 200)
		);
		const current = await api.getInstanceNetwork('web-1');
		expect(current.network).toBe('br01');
		expect(current.verified).toBe(true);

		fetchMock.mockResolvedValueOnce(
			jsonResponse({ name: 'web-1', verified: false }, 200)
		);
		const cleared = await api.setInstanceNetwork('web-1');
		expect(cleared.verified).toBe(false);

		const calls = fetchMock.mock.calls as FetchArgs[];
		expect(calls[1][0]).toBe('/api/instances/web-1/network');
		expect(calls[1][1] && calls[1][1].method).toBe('POST');
		expect(calls[1][1] && calls[1][1].body).toBe(JSON.stringify({ network: '' }));
	});
});
