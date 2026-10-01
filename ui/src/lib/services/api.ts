import type {
	Instance,
	CreateInstanceRequest,
	InstanceList,
	InstanceResponse,
	InstanceStateResponse,
	ImageList,
	NetworkList,
	ErrorResponse,
	SnapshotList,
	SnapshotResponse,
	CreateSnapshotRequest,
	InstanceExport,
	ImportInstanceRequest,
	MountRequest,
	MountResponse,
	UploadResponse,
	ConfigResponse,
	ConfigUpdateRequest,
	ClientIPResponse,
	Job,
	JobResponse,
	JobListResponse,
	HostInfo,
	UpdateResourcesRequest
} from '$lib/types';

const API_BASE = '/api';

/** Client IP is not on the server allowlist. */
export class IPForbiddenError extends Error {
	constructor(message = 'Access denied') {
		super(message);
		this.name = 'IPForbiddenError';
	}
}

/** CSRF session/token problem (expired, missing, or invalid token). */
export class CSRFExpiredError extends Error {
	code: string;

	constructor(code: string, message = 'Session expired. Please try again.') {
		super(message);
		this.name = 'CSRFExpiredError';
		this.code = code;
	}
}

/** Server asked us to slow down. Carries the Retry-After delay in ms. */
export class RateLimitedError extends Error {
	retryAfterMs: number;

	constructor(retryAfterMs: number, message = 'Too many requests. Please try again later.') {
		super(message);
		this.name = 'RateLimitedError';
		this.retryAfterMs = retryAfterMs;
	}
}

/** Fallback wait when a 429 carries no (or an unparsable) Retry-After header. */
const DEFAULT_RETRY_AFTER_MS = 5000;

/**
 * Default budget for JSON API requests. Safe because every minute-scale
 * operation runs as a background job (202 + poll); only stalled connections
 * hit this. Uploads use UPLOAD_TIMEOUT_MS instead (large bodies need it).
 */
const DEFAULT_REQUEST_TIMEOUT_MS = 60000;

/** Matches the server ReadTimeout so large uploads are not cut off client-side. */
const UPLOAD_TIMEOUT_MS = 10 * 60 * 1000;

function parseRetryAfterMs(retryAfter: string | null): number {
	if (retryAfter !== null) {
		const seconds = Number(retryAfter);
		if (Number.isFinite(seconds) && seconds >= 0) {
			return seconds * 1000;
		}
	}
	return DEFAULT_RETRY_AFTER_MS;
}

class ApiService {
	private csrfToken: string | null = null;
	private csrfRefresh: Promise<void> | null = null;

	async initCSRF(): Promise<void> {
		// Single-flight: concurrent 403s share one refresh instead of each
		// minting/racing tokens against each other.
		if (!this.csrfRefresh) {
			this.csrfRefresh = this.fetchCSRFToken().finally(() => {
				this.csrfRefresh = null;
			});
		}
		return this.csrfRefresh;
	}

	private async fetchCSRFToken(): Promise<void> {
		try {
			const response = await fetch(`${API_BASE}/csrf/token`, {
				credentials: 'include'
			});
			if (response.ok) {
				const data = await response.json();
				this.csrfToken = data.csrf_token || null;
			}
		} catch {
			this.csrfToken = null;
		}
	}

	private async request<T>(
		endpoint: string,
		options: RequestInit = {},
		retried = false
	): Promise<T> {
		const method = (options.method || 'GET').toUpperCase();
		const isMutating = ['POST', 'PUT', 'DELETE', 'PATCH'].includes(method);

		const headers: Record<string, string> = {
			'Content-Type': 'application/json'
		};
		if (options.headers) {
			if (options.headers instanceof Headers) {
				options.headers.forEach((value, key) => {
					headers[key] = value;
				});
			} else if (Array.isArray(options.headers)) {
				for (const [key, value] of options.headers) {
					headers[key] = value;
				}
			} else {
				Object.assign(headers, options.headers);
			}
		}

		if (isMutating && this.csrfToken) {
			headers['X-CSRF-Token'] = this.csrfToken;
		}

		// Bound every request unless the caller manages its own signal
		// (e.g. cancellable create flows). Prevents spinners hanging
		// forever on stalled connections.
		let ownedController: AbortController | null = null;
		let ownedTimer: ReturnType<typeof setTimeout> | null = null;
		let signal = options.signal;
		if (!signal) {
			ownedController = new AbortController();
			ownedTimer = setTimeout(() => ownedController?.abort(), DEFAULT_REQUEST_TIMEOUT_MS);
			signal = ownedController.signal;
		}

		let response: Response;
		try {
			response = await fetch(`${API_BASE}${endpoint}`, {
				...options,
				credentials: 'include',
				headers,
				signal
			});
		} catch (e) {
			if (ownedController?.signal.aborted) {
				throw new Error('Request timed out. Please try again.');
			}
			throw e;
		} finally {
			if (ownedTimer !== null) {
				clearTimeout(ownedTimer);
			}
		}

		if (response.status === 403) {
			const err = await this.readForbiddenError(response);
			if (err instanceof CSRFExpiredError && isMutating && !retried) {
				// Token was missing/stale (e.g. raced boot, or rotated by a
				// later GET): refresh once and retry a single time.
				await this.initCSRF();
				return this.request<T>(endpoint, options, true);
			}
			if (err instanceof IPForbiddenError) {
				window.location.href = '/unauthorized';
			}
			throw err;
		}

		this.throwIfRateLimited(response);

		if (!response.ok) {
			const error: ErrorResponse = await response.json();
			throw new Error(error.message || 'An error occurred');
		}

		return response.json();
	}

	/** Throw RateLimitedError for 429 responses; no-op otherwise. */
	private throwIfRateLimited(response: Response): void {
		if (response.status === 429) {
			throw new RateLimitedError(parseRetryAfterMs(response.headers.get('Retry-After')));
		}
	}

	/**
	 * Classify a 403 response without side effects. Never throws and never
	 * redirects — callers decide. Tolerates empty/non-JSON bodies.
	 */
	private async readForbiddenError(response: Response): Promise<Error> {
		let code = '';
		let message = '';
		try {
			const data = (await response.json()) as Partial<ErrorResponse>;
			code = data.error || '';
			message = data.message || '';
		} catch {
			// Non-JSON or empty body — fall through to the generic error.
		}

		return this.classifyForbiddenError(code, message);
	}

	/**
	 * Map a parsed 403 error code/message to an Error. Shared by the fetch
	 * and XHR paths so both classify denials identically.
	 */
	private classifyForbiddenError(code: string, message: string): Error {
		if (code === 'forbidden') {
			return new IPForbiddenError(message || 'Access denied');
		}
		if (code.startsWith('csrf_')) {
			return new CSRFExpiredError(code, message || 'Session expired. Please try again.');
		}
		return new Error(message || 'Access denied');
	}

	async getInstances(): Promise<Instance[]> {
		const data = await this.request<InstanceList>('/instances');
		return data.instances;
	}

	async getInstance(name: string, signal?: AbortSignal): Promise<Instance> {
		return this.request<Instance>(`/instances/${encodeURIComponent(name)}`, { signal });
	}

	async getInstanceState(name: string): Promise<InstanceStateResponse> {
		return this.request<InstanceStateResponse>(`/instances/${encodeURIComponent(name)}/state`);
	}

	async createInstance(request: CreateInstanceRequest): Promise<Instance> {
		return this.request<Instance>('/instances', {
			method: 'POST',
			body: JSON.stringify(request)
		});
	}

	async createInstanceAsync(
		request: CreateInstanceRequest,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		const headers: Record<string, string> = {};
		if (options.idempotencyKey) {
			headers['Idempotency-Key'] = options.idempotencyKey;
		}
		const data = await this.request<JobResponse>('/instances/async', {
			method: 'POST',
			body: JSON.stringify(request),
			headers,
			signal: options.signal
		});
		return data.job;
	}

	async getJob(id: string, signal?: AbortSignal): Promise<Job> {
		const data = await this.request<JobResponse>(`/jobs/${id}`, { signal });
		return data.job;
	}

	async listJobs(): Promise<Job[]> {
		const data = await this.request<JobListResponse>('/jobs');
		return data.jobs;
	}

	async deleteInstance(name: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(`/instances/${encodeURIComponent(name)}`, {
			method: 'DELETE'
		});
	}

	async startInstance(name: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(`/instances/${encodeURIComponent(name)}/start`, {
			method: 'POST'
		});
	}

	async stopInstance(name: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(`/instances/${encodeURIComponent(name)}/stop`, {
			method: 'POST'
		});
	}

	async restartInstance(name: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(`/instances/${encodeURIComponent(name)}/restart`, {
			method: 'POST'
		});
	}

	async recoverInstance(name: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(`/instances/${encodeURIComponent(name)}/recover`, {
			method: 'POST'
		});
	}

	async purgeDeletedInstances(): Promise<InstanceResponse> {
		return this.request<InstanceResponse>('/instances/purge', {
			method: 'POST'
		});
	}

	async getImages(): Promise<ImageList> {
		return this.request<ImageList>('/images');
	}

	async getNetworks(): Promise<NetworkList> {
		return this.request<NetworkList>('/networks');
	}

	async createNetwork(name: string, mode?: string, mac?: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>('/networks', {
			method: 'POST',
			body: JSON.stringify({ name, mode, mac })
		});
	}

	async deleteNetwork(name: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(`/networks/${encodeURIComponent(name)}`, {
			method: 'DELETE'
		});
	}

	async healthCheck(): Promise<{ status: string }> {
		return this.request<{ status: string }>('/health');
	}

	async createSnapshot(
		instanceName: string,
		request: CreateSnapshotRequest
	): Promise<SnapshotResponse> {
		return this.request<SnapshotResponse>(
			`/instances/${encodeURIComponent(instanceName)}/snapshots`,
			{
				method: 'POST',
				body: JSON.stringify(request)
			}
		);
	}

	async getSnapshots(instanceName: string): Promise<SnapshotList> {
		return this.request<SnapshotList>(`/instances/${encodeURIComponent(instanceName)}/snapshots`);
	}

	async restoreSnapshot(instanceName: string, snapshotName: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(
			`/instances/${encodeURIComponent(instanceName)}/snapshots/${encodeURIComponent(snapshotName)}/restore`,
			{
				method: 'POST'
			}
		);
	}

	async deleteSnapshot(instanceName: string, snapshotName: string): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(
			`/instances/${encodeURIComponent(instanceName)}/snapshots/${encodeURIComponent(snapshotName)}`,
			{
				method: 'DELETE'
			}
		);
	}

	async exportInstance(instanceName: string, outputPath?: string): Promise<InstanceExport> {
		return this.request<InstanceExport>(`/instances/${encodeURIComponent(instanceName)}/export`, {
			method: 'POST',
			body: JSON.stringify({ output_path: outputPath })
		});
	}

	async importInstance(request: ImportInstanceRequest): Promise<Instance> {
		return this.request<Instance>('/instances/import', {
			method: 'POST',
			body: JSON.stringify(request)
		});
	}

	async mountInstance(instanceName: string, request: MountRequest): Promise<MountResponse> {
		return this.request<MountResponse>(`/instances/${encodeURIComponent(instanceName)}/mounts`, {
			method: 'POST',
			body: JSON.stringify(request)
		});
	}

	/**
	 * Dispatch a long-running operation as a background job. Returns
	 * immediately (202); track completion via getJob() or the jobs store
	 * (SSE + polling). Pass a fresh idempotency key per user action so
	 * network retries of the same dispatch collapse onto one job.
	 */
	private async postAsyncJob(
		url: string,
		body: unknown,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		const headers: Record<string, string> = {};
		if (options.idempotencyKey) {
			headers['Idempotency-Key'] = options.idempotencyKey;
		}
		const data = await this.request<JobResponse>(url, {
			method: 'POST',
			body: JSON.stringify(body),
			headers,
			signal: options.signal
		});
		return data.job;
	}

	async mountInstanceAsync(
		instanceName: string,
		request: MountRequest,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		return this.postAsyncJob(
			`/instances/${encodeURIComponent(instanceName)}/mounts/async`,
			request,
			options
		);
	}

	async createSnapshotAsync(
		instanceName: string,
		request: CreateSnapshotRequest,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		return this.postAsyncJob(
			`/instances/${encodeURIComponent(instanceName)}/snapshots/async`,
			request,
			options
		);
	}

	async restoreSnapshotAsync(
		instanceName: string,
		snapshotName: string,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		return this.postAsyncJob(
			`/instances/${encodeURIComponent(instanceName)}/snapshots/${encodeURIComponent(snapshotName)}/restore/async`,
			{},
			options
		);
	}

	async exportInstanceAsync(
		instanceName: string,
		outputPath?: string,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		return this.postAsyncJob(
			`/instances/${encodeURIComponent(instanceName)}/export/async`,
			{ output_path: outputPath },
			options
		);
	}

	async importInstanceAsync(
		request: ImportInstanceRequest,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		return this.postAsyncJob('/instances/import/async', request, options);
	}

	async unmountInstance(instanceName: string, targetPath: string): Promise<MountResponse> {
		return this.request<MountResponse>(`/instances/${encodeURIComponent(instanceName)}/mounts`, {
			method: 'DELETE',
			body: JSON.stringify({ target_path: targetPath })
		});
	}

	async uploadFile(
		instanceName: string,
		file: File,
		targetPath?: string,
		retried = false
	): Promise<UploadResponse> {
		const formData = new FormData();
		formData.append('file', file);
		if (targetPath) {
			formData.append('target_path', targetPath);
		}

		const response = await this.postFormData(
			`${API_BASE}/instances/${encodeURIComponent(instanceName)}/upload`,
			formData,
			retried
		);

		if (!response.ok) {
			throw new Error((await this.readUploadErrorMessage(response)) || 'Upload failed');
		}

		return response.json();
	}

	/**
	 * POST multipart form data with the shared semantics: credentials,
	 * CSRF header plus a single refresh-and-retry, IP-denial redirect,
	 * and rate-limit mapping. Callers parse the body themselves.
	 * Note: no Content-Type header — the browser sets the multipart
	 * boundary automatically.
	 */
	private async postFormData(
		url: string,
		formData: FormData,
		retried = false,
		extraHeaders: Record<string, string> = {}
	): Promise<Response> {
		const headers: Record<string, string> = { ...extraHeaders };
		if (this.csrfToken) {
			headers['X-CSRF-Token'] = this.csrfToken;
		}

		const response = await fetch(url, {
			method: 'POST',
			body: formData,
			credentials: 'include',
			headers
		});

		if (response.status === 403) {
			const err = await this.readForbiddenError(response);
			if (err instanceof CSRFExpiredError && !retried) {
				await this.initCSRF();
				return this.postFormData(url, formData, true, extraHeaders);
			}
			if (err instanceof IPForbiddenError) {
				window.location.href = '/unauthorized';
			}
			throw err;
		}

		if (response.status === 429) {
			this.throwIfRateLimited(response);
		}

		return response;
	}

	/**
	 * Upload a VM image from the browser and launch it as a job.
	 * Returns immediately (202); track via getJob() or the jobs store.
	 */
	async importInstanceFromFile(
		file: File,
		name?: string,
		options: { idempotencyKey?: string; signal?: AbortSignal } = {}
	): Promise<Job> {
		const formData = new FormData();
		formData.append('file', file);
		if (name) {
			formData.append('name', name);
		}

		const extraHeaders: Record<string, string> = {};
		if (options.idempotencyKey) {
			extraHeaders['Idempotency-Key'] = options.idempotencyKey;
		}
		const response = await this.postFormData(
			`${API_BASE}/instances/import/async`,
			formData,
			false,
			extraHeaders
		);

		if (!response.ok) {
			throw new Error((await this.readUploadErrorMessage(response)) || 'Import failed');
		}

		const data = (await response.json()) as JobResponse;
		return data.job;
	}

	/**
	 * Browser-navigation URL for downloading a completed export (or its
	 * sidecar). Downloads ride cookies, and GET needs no CSRF header.
	 */
	exportInstanceDownloadUrl(instanceName: string, jobId: string, sidecar = false): string {
		const params = new URLSearchParams({ job_id: jobId });
		if (sidecar) {
			params.set('sidecar', 'true');
		}
		return `${API_BASE}/instances/${encodeURIComponent(instanceName)}/export/download?${params.toString()}`;
	}

	/**
	 * Read an upload failure message without throwing. Upload error bodies
	 * may be empty or non-JSON (e.g. from proxies or body-limit rejections),
	 * so fall back to the status text.
	 */
	private async readUploadErrorMessage(response: Response): Promise<string> {
		try {
			const error = (await response.json()) as Partial<ErrorResponse>;
			if (error && typeof error.message === 'string' && error.message !== '') {
				return error.message;
			}
		} catch {
			// Non-JSON or empty body — fall through to the status text.
		}
		return response.statusText || '';
	}

	/**
	 * Upload a file with progress reporting. `fetch` cannot observe upload
	 * progress, so this uses XMLHttpRequest with identical semantics to
	 * `uploadFile` (credentials, CSRF header + single retry, IP redirect,
	 * rate-limit mapping, tolerant error parsing). `onProgress` receives
	 * loaded/total bytes; it is not called when the total is unknown.
	 */
	async uploadFileWithProgress(
		instanceName: string,
		file: File,
		targetPath?: string,
		onProgress?: (loaded: number, total: number) => void
	): Promise<UploadResponse> {
		try {
			return await this.xhrUpload(instanceName, file, targetPath, onProgress);
		} catch (e) {
			if (e instanceof CSRFExpiredError) {
				await this.initCSRF();
				return this.xhrUpload(instanceName, file, targetPath, onProgress);
			}
			throw e;
		}
	}

	private xhrUpload(
		instanceName: string,
		file: File,
		targetPath?: string,
		onProgress?: (loaded: number, total: number) => void
	): Promise<UploadResponse> {
		return new Promise((resolve, reject) => {
			const formData = new FormData();
			formData.append('file', file);
			if (targetPath) {
				formData.append('target_path', targetPath);
			}

			const xhr = new XMLHttpRequest();
			xhr.open('POST', `${API_BASE}/instances/${encodeURIComponent(instanceName)}/upload`);
			xhr.withCredentials = true;
			// Large bodies need the same budget as the server ReadTimeout.
			xhr.timeout = UPLOAD_TIMEOUT_MS;
			if (this.csrfToken) {
				xhr.setRequestHeader('X-CSRF-Token', this.csrfToken);
			}
			if (onProgress && xhr.upload) {
				xhr.upload.onprogress = (event: ProgressEvent) => {
					if (event.lengthComputable) {
						onProgress(event.loaded, event.total);
					}
				};
			}
			xhr.onload = () => {
				if (xhr.status === 403) {
					const err = this.classifyXHRForbidden(xhr);
					if (err instanceof IPForbiddenError) {
						window.location.href = '/unauthorized';
					}
					reject(err);
					return;
				}
				if (xhr.status === 429) {
					reject(new RateLimitedError(parseRetryAfterMs(xhr.getResponseHeader('Retry-After'))));
					return;
				}
				if (xhr.status >= 200 && xhr.status < 300) {
					try {
						resolve(JSON.parse(xhr.responseText) as UploadResponse);
					} catch {
						reject(new Error('Upload failed'));
					}
					return;
				}
				reject(new Error(this.readXHRMessage(xhr) || 'Upload failed'));
			};
			xhr.onerror = () => {
				reject(new Error('Upload failed'));
			};
			xhr.ontimeout = () => {
				reject(new Error('Upload timed out. Please try again.'));
			};
			xhr.send(formData);
		});
	}

	/** Parse an XHR 403 body with the same classification as fetch denials. */
	private classifyXHRForbidden(xhr: XMLHttpRequest): Error {
		const body = this.readXHRError(xhr);
		return this.classifyForbiddenError(body.error, body.message);
	}

	private readXHRError(xhr: XMLHttpRequest): { error: string; message: string } {
		try {
			const data = JSON.parse(xhr.responseText) as Partial<ErrorResponse>;
			return { error: data.error || '', message: data.message || '' };
		} catch {
			return { error: '', message: '' };
		}
	}

	/** Read an XHR failure message without throwing on empty/non-JSON bodies. */
	private readXHRMessage(xhr: XMLHttpRequest): string {
		return this.readXHRError(xhr).message || xhr.statusText || '';
	}

	async getConfig(): Promise<ConfigResponse> {
		return this.request<ConfigResponse>('/config');
	}

	async updateConfig(request: ConfigUpdateRequest): Promise<InstanceResponse> {
		return this.request<InstanceResponse>('/config', {
			method: 'POST',
			body: JSON.stringify(request)
		});
	}

	/**
	 * Returns this client's IP address as the server evaluates it for
	 * allowlist enforcement. Used to warn before saving a list that would
	 * lock the current admin out.
	 */
	async getClientIP(): Promise<string> {
		const data = await this.request<ClientIPResponse>('/config/client-ip');
		return data.ip;
	}

	async getHostInfo(): Promise<HostInfo> {
		return this.request<HostInfo>('/host');
	}

	async updateInstanceResources(
		instanceName: string,
		request: UpdateResourcesRequest
	): Promise<InstanceResponse> {
		return this.request<InstanceResponse>(
			`/instances/${encodeURIComponent(instanceName)}/resources`,
			{
				method: 'PUT',
				body: JSON.stringify(request)
			}
		);
	}
}

export const api = new ApiService();
