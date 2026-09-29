import type {
	Instance,
	CreateInstanceRequest,
	InstanceList,
	InstanceResponse,
	InstanceState,
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

function parseRetryAfterMs(response: Response): number {
	const raw = response.headers.get('Retry-After');
	if (raw !== null) {
		const seconds = Number(raw);
		if (Number.isFinite(seconds) && seconds >= 0) {
			return seconds * 1000;
		}
	}
	return DEFAULT_RETRY_AFTER_MS;
}

class ApiService {
	private csrfToken: string | null = null;

	async initCSRF(): Promise<void> {
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
			'Content-Type': 'application/json',
			...options.headers
		};

		if (isMutating && this.csrfToken) {
			headers['X-CSRF-Token'] = this.csrfToken;
		}

		const response = await fetch(`${API_BASE}${endpoint}`, {
			...options,
			credentials: 'include',
			headers
		});

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

		if (response.status === 429) {
			throw new RateLimitedError(parseRetryAfterMs(response));
		}

		if (!response.ok) {
			const error: ErrorResponse = await response.json();
			throw new Error(error.message || 'An error occurred');
		}

		return response.json();
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

	async getInstanceState(name: string): Promise<InstanceState> {
		return this.request<InstanceState>(`/instances/${encodeURIComponent(name)}/state`);
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

		// Note: no Content-Type header — the browser sets the multipart
		// boundary automatically.
		const headers: Record<string, string> = {};
		if (this.csrfToken) {
			headers['X-CSRF-Token'] = this.csrfToken;
		}

		const response = await fetch(
			`${API_BASE}/instances/${encodeURIComponent(instanceName)}/upload`,
			{
				method: 'POST',
				body: formData,
				credentials: 'include',
				headers
			}
		);

		if (response.status === 403) {
			const err = await this.readForbiddenError(response);
			if (err instanceof CSRFExpiredError && !retried) {
				await this.initCSRF();
				return this.uploadFile(instanceName, file, targetPath, true);
			}
			if (err instanceof IPForbiddenError) {
				window.location.href = '/unauthorized';
			}
			throw err;
		}

		if (!response.ok) {
			const error = await response.json();
			throw new Error(error.message || 'Upload failed');
		}

		return response.json();
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
