import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api } from '../src/lib/services/api';

vi.mock('$lib/services/api', async () => {
	const actual = await vi.importActual<typeof import('../src/lib/services/api')>('../src/lib/services/api');
	return {
		api: actual.api
	};
});

describe('ApiService', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it('should construct API_BASE correctly', () => {
		expect(api).toBeDefined();
		expect(typeof api.getInstances).toBe('function');
		expect(typeof api.createInstance).toBe('function');
		expect(typeof api.deleteInstance).toBe('function');
	});

	it('should have all required methods', () => {
		const requiredMethods = [
			'getInstances',
			'getInstance',
			'getInstanceState',
			'createInstance',
			'createInstanceAsync',
			'getJob',
			'listJobs',
			'deleteInstance',
			'startInstance',
			'stopInstance',
			'restartInstance',
			'getImages',
			'getNetworks',
			'createNetwork',
			'deleteNetwork',
			'healthCheck',
			'createSnapshot',
			'getSnapshots',
			'restoreSnapshot',
			'deleteSnapshot',
			'exportInstance',
			'importInstance',
			'mountInstance',
			'unmountInstance',
			'uploadFile',
			'getConfig',
			'updateConfig',
			'getHostInfo',
			'updateInstanceResources'
		];

		for (const method of requiredMethods) {
			expect(typeof (api as Record<string, unknown>)[method]).toBe('function');
		}
	});
});
