import type {
	Instance,
	CreateInstanceRequest,
	InstanceList,
	InstanceResponse,
	ImageList,
	NetworkList,
	ErrorResponse
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

	async createInstance(request: CreateInstanceRequest): Promise<Instance> {
		return this.request<Instance>('/instances', {
			method: 'POST',
			body: JSON.stringify(request)
		});
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

	async healthCheck(): Promise<{ status: string }> {
		return this.request<{ status: string }>('/health');
	}
}

export const api = new ApiService();
