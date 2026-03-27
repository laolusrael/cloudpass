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
