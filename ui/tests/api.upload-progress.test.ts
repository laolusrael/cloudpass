import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { api, IPForbiddenError, CSRFExpiredError, RateLimitedError } from '../src/lib/services/api';

type ProgressHandler = (event: {
	loaded: number;
	total: number;
	lengthComputable: boolean;
}) => void;

class FakeXHR {
	static instances: FakeXHR[] = [];

	upload: { onprogress: ProgressHandler | null } = { onprogress: null };
	withCredentials = false;
	requestHeaders: Record<string, string> = {};
	status = 0;
	responseText = '';
	statusText = '';
	responseHeaders: Record<string, string> = {};
	timeout = 0;
	onload: (() => void) | null = null;
	onerror: (() => void) | null = null;
	ontimeout: (() => void) | null = null;
	sentBody: unknown = null;
	method = '';
	url = '';

	constructor() {
		FakeXHR.instances.push(this);
	}

	open(method: string, url: string): void {
		this.method = method;
		this.url = url;
	}

	setRequestHeader(name: string, value: string): void {
		this.requestHeaders[name] = value;
	}

	getResponseHeader(name: string): string | null {
		return this.responseHeaders[name] ?? null;
	}

	send(body: unknown): void {
		this.sentBody = body;
	}

	respond(status: number, body: string, headers: Record<string, string> = {}): void {
		this.status = status;
		this.responseText = body;
		this.responseHeaders = headers;
		this.onload?.();
	}

	failNetwork(): void {
		this.onerror?.();
	}

	expire(): void {
		this.ontimeout?.();
	}

	emitProgress(loaded: number, total: number): void {
		this.upload.onprogress?.({ loaded, total, lengthComputable: true });
	}
}

function setCsrfToken(value: string | null): void {
	(api as unknown as { csrfToken: string | null }).csrfToken = value;
}

function flush(): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, 0));
}

describe('ApiService uploadFileWithProgress', () => {
	const fetchMock = vi.fn();
	const originalLocationDescriptor = Object.getOwnPropertyDescriptor(window, 'location');
	let redirectTarget: { href: string };

	beforeEach(() => {
		FakeXHR.instances = [];
		vi.stubGlobal('XMLHttpRequest', FakeXHR as unknown as typeof XMLHttpRequest);
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
		setCsrfToken(null);
	});

	it('sends credentials and CSRF header, resolving the parsed body', async () => {
		setCsrfToken('up-tok');
		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file, '/home/ubuntu/');

		expect(FakeXHR.instances).toHaveLength(1);
		const xhr = FakeXHR.instances[0];
		expect(xhr.method).toBe('POST');
		expect(xhr.url).toBe('/api/instances/web-1/upload');
		expect(xhr.withCredentials).toBe(true);
		expect(xhr.requestHeaders['X-CSRF-Token']).toBe('up-tok');
		expect((xhr.sentBody as FormData).get('target_path')).toBe('/home/ubuntu/');

		xhr.respond(201, JSON.stringify({ message: 'File uploaded', path: '/home/ubuntu/seed.txt' }));
		await expect(pending).resolves.toEqual({
			message: 'File uploaded',
			path: '/home/ubuntu/seed.txt'
		});
	});

	it('reports upload progress as loaded/total', async () => {
		const seen: Array<[number, number]> = [];
		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file, undefined, (loaded, total) => {
			seen.push([loaded, total]);
		});

		FakeXHR.instances[0].emitProgress(50, 100);
		FakeXHR.instances[0].emitProgress(100, 100);
		expect(seen).toEqual([
			[50, 100],
			[100, 100]
		]);

		FakeXHR.instances[0].respond(201, JSON.stringify({ message: 'ok', path: '/seed.txt' }));
		await expect(pending).resolves.toEqual({ message: 'ok', path: '/seed.txt' });
	});

	it('retries once with a fresh token after CSRF expiry', async () => {
		setCsrfToken('expired');
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ csrf_token: 'fresh' }), { status: 200 })
		);

		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file);

		FakeXHR.instances[0].respond(
			403,
			JSON.stringify({ error: 'csrf_token_missing', message: 'CSRF token is required' })
		);
		await flush();
		await flush();

		expect(FakeXHR.instances).toHaveLength(2);
		expect(FakeXHR.instances[1].requestHeaders['X-CSRF-Token']).toBe('fresh');

		FakeXHR.instances[1].respond(201, JSON.stringify({ message: 'ok', path: '/seed.txt' }));
		await expect(pending).resolves.toEqual({ message: 'ok', path: '/seed.txt' });
		expect(redirectTarget.href).toBe('');
	});

	it('redirects and rejects on IP denial', async () => {
		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file);

		FakeXHR.instances[0].respond(
			403,
			JSON.stringify({ error: 'forbidden', message: 'access denied' })
		);
		await expect(pending).rejects.toThrow(IPForbiddenError);
		expect(redirectTarget.href).toBe('/unauthorized');
	});

	it('rejects unretryable CSRF errors without redirect', async () => {
		fetchMock.mockResolvedValueOnce(
			new Response(JSON.stringify({ csrf_token: 'fresh' }), { status: 200 })
		);

		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file);

		// First attempt fails, retry refresh succeeds, second attempt fails again.
		FakeXHR.instances[0].respond(
			403,
			JSON.stringify({ error: 'csrf_token_invalid', message: 'bad token' })
		);
		await flush();
		await flush();
		expect(FakeXHR.instances).toHaveLength(2);
		FakeXHR.instances[1].respond(
			403,
			JSON.stringify({ error: 'csrf_token_invalid', message: 'bad token' })
		);
		await expect(pending).rejects.toThrow(CSRFExpiredError);
		expect(redirectTarget.href).toBe('');
	});

	it('maps 429 to RateLimitedError with the header delay', async () => {
		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file);

		FakeXHR.instances[0].respond(429, JSON.stringify({ error: 'rate_limited' }), {
			'Retry-After': '45'
		});
		const err = await pending.catch((e: unknown) => e);
		expect(err).toBeInstanceOf(RateLimitedError);
		expect((err as RateLimitedError).retryAfterMs).toBe(45000);
	});

	it('surfaces backend messages and tolerates non-JSON bodies', async () => {
		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });

		const first = api.uploadFileWithProgress('web-1', file);
		FakeXHR.instances[0].respond(
			400,
			JSON.stringify({ error: 'file_too_large', message: 'file size exceeds maximum' })
		);
		await expect(first).rejects.toThrow('file size exceeds maximum');

		const second = api.uploadFileWithProgress('web-1', file);
		FakeXHR.instances[1].respond(500, '');
		await expect(second).rejects.toThrow('Upload failed');
	});

	it('rejects on network failure', async () => {
		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file);

		FakeXHR.instances[0].failNetwork();
		await expect(pending).rejects.toThrow('Upload failed');
	});

	it('sets a generous timeout and rejects on expiry', async () => {
		const file = new File(['data'], 'seed.txt', { type: 'text/plain' });
		const pending = api.uploadFileWithProgress('web-1', file);

		expect(FakeXHR.instances[0].timeout).toBe(10 * 60 * 1000);
		FakeXHR.instances[0].expire();
		await expect(pending).rejects.toThrow('Upload timed out');
	});
});
