import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import { networks } from '../src/lib/stores/networks';
import type { Network } from '../src/lib/types';

const { mockGetNetworks } = vi.hoisted(() => ({
	mockGetNetworks: vi.fn()
}));

vi.mock('$lib/services/api', async () => {
	const actual =
		await vi.importActual<typeof import('../src/lib/services/api')>('../src/lib/services/api');
	return {
		...actual,
		api: {
			...actual.api,
			getNetworks: mockGetNetworks
		}
	};
});

function network(name: string, extra: Partial<Network> = {}): Network {
	return { name, type: 'bridge', ipv4: '', description: '', ...extra };
}

describe('networks store', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('loads networks and unverified instances', async () => {
		mockGetNetworks.mockResolvedValue({
			networks: [network('br01', { used_by: ['web-1'], managed: true })],
			unverified_instances: ['old']
		});

		await networks.refresh();

		expect(get(networks)).toEqual([network('br01', { used_by: ['web-1'], managed: true })]);
		expect(get(networks.unverified)).toEqual(['old']);
		expect(get(networks.loading)).toBe(false);
		expect(get(networks.error)).toBeNull();
	});

	it('normalizes a null payload to empty lists', async () => {
		mockGetNetworks.mockResolvedValue({ networks: null, unverified_instances: null });

		await networks.refresh();

		expect(get(networks)).toEqual([]);
		expect(get(networks.unverified)).toEqual([]);
		expect(get(networks.error)).toBeNull();
	});

	it('surfaces load failures on the error store', async () => {
		mockGetNetworks.mockRejectedValue(new Error('daemon down'));

		await networks.refresh();

		expect(get(networks.error)).toBe('daemon down');
		expect(get(networks.loading)).toBe(false);
	});
});
