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
	ConfigResponse,
	ConfigUpdateRequest,
	Job,
	JobResponse
} from '$lib/types';

const API_BASE = '/api';

class ApiService {
	private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
		const response = await fetch(`${API_BASE}${endpoint}`, {
			...options,
			headers: {
				'Content-Type': 'application/json',
				...options.headers
			}
		});

		if (!response.ok) {
			const error: ErrorResponse = await response.json();
			throw new Error(error.message || 'An error occurred');
		}

		return response.json();
	}

	async getInstances(): Promise<Instance[]> {
		const data = await this.request<InstanceList>('/instances');
		return data.instances;
	}

	async getInstance(name: string): Promise<Instance> {
		return this.request<Instance>(`/instances/${encodeURIComponent(name)}`);
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

	async createInstanceAsync(request: CreateInstanceRequest): Promise<Job> {
		const data = await this.request<JobResponse>('/instances/async', {
			method: 'POST',
			body: JSON.stringify(request)
		});
		return data.job;
	}

	async getJob(id: string): Promise<Job> {
		const data = await this.request<JobResponse>(`/jobs/${id}`);
		return data.job;
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

	async createSnapshot(instanceName: string, request: CreateSnapshotRequest): Promise<SnapshotResponse> {
		return this.request<SnapshotResponse>(`/instances/${encodeURIComponent(instanceName)}/snapshots`, {
			method: 'POST',
			body: JSON.stringify(request)
		});
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

	async getConfig(): Promise<ConfigResponse> {
		return this.request<ConfigResponse>('/config');
	}

	async updateConfig(request: ConfigUpdateRequest): Promise<InstanceResponse> {
		return this.request<InstanceResponse>('/config', {
			method: 'POST',
			body: JSON.stringify(request)
		});
	}
}

export const api = new ApiService();
