import { describe, it, expect, vi, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { instances } from '../src/lib/stores/instances';

vi.mock('$lib/services/api', () => ({
	api: {
		getInstances: vi.fn(),
		getInstance: vi.fn(),
		createInstance: vi.fn(),
		deleteInstance: vi.fn(),
		startInstance: vi.fn(),
		stopInstance: vi.fn(),
		restartInstance: vi.fn()
	}
}));

describe('instances store', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('should have initial empty state', () => {
		const current = get(instances);
		expect(current).toEqual([]);
	});

	it('should have loading and error properties', () => {
		expect(instances.loading).toBeDefined();
		expect(instances.error).toBeDefined();
		expect(instances.refresh).toBeDefined();
		expect(instances.create).toBeDefined();
		expect(instances.delete).toBeDefined();
		expect(instances.start).toBeDefined();
		expect(instances.stop).toBeDefined();
		expect(instances.restart).toBeDefined();
	});

	it('should have derived stores', () => {
		expect(instances).toBeDefined();
	});
});
