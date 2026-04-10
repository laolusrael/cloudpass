export interface Instance {
	name: string;
	state: string;
	ipv4: string[];
	ipv6: string[];
	cpu: number;
	memory: string;
	disk: string;
	image: string;
	release: string;
	created_at: string;
	mounts: Mount[];
	disks: Disk[];
	load: number[];
	network: Record<string, NetworkInfo>;
}

export interface Mount {
	source: string;
	target: string;
}

export interface Disk {
	name: string;
	total: string;
	used: string;
}

export interface NetworkInfo {
	ipv4: string;
	ipv6: string;
}

export interface CreateInstanceRequest {
	name: string;
	image?: string;
	cpus?: number;
	memory?: string;
	disk?: string;
	network?: string;
	cloud_init?: string;
}

export interface Image {
	alias: string;
	version: string;
	release: string;
	remote: string;
	os: string;
	aliases: string[];
}

export interface Network {
	name: string;
	type: string;
	ipv4: string;
	description: string;
}

export interface InstanceList {
	instances: Instance[];
}

export interface ImageList {
	images: Image[];
}

export interface NetworkList {
	networks: Network[];
}

export interface InstanceResponse {
	message: string;
}

export interface Job {
	id: string;
	type: string;
	status: 'pending' | 'running' | 'completed' | 'failed';
	instance_name?: string;
	error?: string;
	created_at: string;
	updated_at: string;
}

export interface JobResponse {
	job: Job;
}

export interface JobListResponse {
	jobs: Job[];
}

export interface ErrorResponse {
	error: string;
	message: string;
}

export type InstanceState =
	| 'Starting'
	| 'Running'
	| 'Stopping'
	| 'Stopped'
	| 'Suspended'
	| 'Deleting'
	| 'Deleted';

export interface Snapshot {
	name: string;
	instance: string;
	created_at?: string;
	comment?: string;
	parent?: string;
	children?: number;
	state_size?: number;
	disk_size?: number;
	memory_size?: number;
}

export interface SnapshotList {
	snapshots: Snapshot[];
}

export interface SnapshotResponse {
	message: string;
	snapshot_name: string;
	instance_name: string;
	instance_stopped: boolean;
	instance_started?: boolean;
}

export interface InstanceState {
	name: string;
	state: string;
}

export interface SnapshotList {
	snapshots: Snapshot[];
}

export interface CreateSnapshotRequest {
	name?: string;
	comment?: string;
}

export interface RestoreSnapshotRequest {
	name?: string;
}

export interface ExportInstanceRequest {
	output_path?: string;
	format?: string;
}

export interface ImportInstanceRequest {
	name?: string;
	image_path: string;
	cpus?: number;
	memory?: string;
	disk?: string;
}

export interface InstanceExport {
	message: string;
	image_path?: string;
}

export interface MountRequest {
	source_path: string;
	target_path: string;
}

export interface UnmountRequest {
	target_path: string;
}

export interface MountResponse {
	message?: string;
	source?: string;
	target?: string;
}

export interface UploadResponse {
	message?: string;
	path?: string;
}

export interface ConfigResponse {
	server: {
		host: string;
		port: number;
	};
	security: {
		allowed_ips: string[];
		websocket_timeout_minutes: number;
	};
	multipass: {
		socket_path: string;
		default_timeout_seconds: number;
		ssh_key_path: string;
	};
	upload?: {
		max_file_size_mb: number;
		default_path: string;
	};
	logging: {
		level: string;
		format: string;
		output: string;
	};
}

export interface ConfigUpdateRequest {
	server?: {
		host?: string;
		port?: number;
	};
	security?: {
		allowed_ips?: string[];
		websocket_timeout_minutes?: number;
	};
	multipass?: {
		socket_path?: string;
		default_timeout_seconds?: number;
		ssh_key_path?: string;
	};
	upload?: {
		max_file_size_mb?: number;
		default_path?: string;
	};
	logging?: {
		level?: string;
		format?: string;
		output?: string;
	};
}

export interface HostInfo {
	cpu_cores: number;
	cpu_available: number;
	cpu_reserved: number;
	memory_bytes: number;
	memory_available: number;
	memory_reserved: number;
	disk_bytes: number;
	disk_available: number;
}

export interface UpdateResourcesRequest {
	cpus: number;
	memory: string;
	disk: string;
}
