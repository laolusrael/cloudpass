import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api } from '../src/lib/services/api';

function setCsrfToken(value: string | null): void {
	(api as unknown as { csrfToken: string | null }).csrfToken = value;
}

describe('ApiService import/export transfer', () => {
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

	it('uploads an image file and resolves the job', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ job: { id: 'job-1', status: 'pending' } }), {
				status: 202
			})
		);

		const file = new File(['fake-image'], 'web-server.img', {
			type: 'application/octet-stream'
		});
		const job = await api.importInstanceFromFile(file, 'web-server', {
			idempotencyKey: 'key-123'
		});

		expect(job).toEqual({ id: 'job-1', status: 'pending' });
		const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
		expect(url).toBe('/api/instances/import/async');
		const body = init.body as FormData;
		expect((body.get('file') as File).name).toBe('web-server.img');
		expect(body.get('name')).toBe('web-server');
		expect((init.headers as Record<string, string>)['Idempotency-Key']).toBe('key-123');
	});

	it('surfaces duplicate instance conflicts', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(
				JSON.stringify({ error: 'conflict', message: 'already exists (state: Running)' }),
				{ status: 409 }
			)
		);

		const file = new File(['x'], 'a.img', { type: 'application/octet-stream' });
		await expect(api.importInstanceFromFile(file)).rejects.toThrow('already exists');
	});

	it('builds download URLs', () => {
		expect(api.exportInstanceDownloadUrl('web-1', 'job-9')).toBe(
			'/api/instances/web-1/export/download?job_id=job-9'
		);
		expect(api.exportInstanceDownloadUrl('web-1', 'job-9', true)).toBe(
			'/api/instances/web-1/export/download?job_id=job-9&sidecar=true'
		);
	});
});
