import { describe, it, expect } from 'vitest';
import {
	parseAllowedEntry,
	validateAllowedEntry,
	allowlistCoversIp,
	decideSave
} from '../src/lib/validation/ip';

describe('parseAllowedEntry', () => {
	it('passes CIDR notation through', () => {
		expect(parseAllowedEntry('192.168.1.0/24')?.normalized).toBe('192.168.1.0/24');
		expect(parseAllowedEntry('10.0.0.0/8')?.kind).toBe('cidr');
	});

	it('normalizes bare IPs to host routes', () => {
		expect(parseAllowedEntry('192.168.1.100')).toMatchObject({
			normalized: '192.168.1.100/32',
			kind: 'host',
			family: 4
		});
		expect(parseAllowedEntry('::1')).toMatchObject({
			normalized: '::1/128',
			kind: 'host',
			family: 6
		});
	});

	it('trims whitespace and strips zones', () => {
		expect(parseAllowedEntry('  10.0.0.5  ')?.normalized).toBe('10.0.0.5/32');
		expect(parseAllowedEntry('fe80::1%eth0')?.normalized).toBe('fe80::1/128');
		expect(parseAllowedEntry('fe80::1%eth0/64')?.normalized).toBe('fe80::1/64');
	});

	it('rejects garbage', () => {
		for (const bad of [
			'',
			'   ',
			'not-an-ip',
			'999.999.0.0/16',
			'192.168.1.0/33',
			'10.0.0.1/abc',
			'example.com'
		]) {
			expect(parseAllowedEntry(bad)).toBeNull();
			expect(validateAllowedEntry(bad)).not.toBeNull();
		}
	});

	it('accepts valid entries without error', () => {
		for (const good of ['192.168.1.0/24', '192.168.1.100', '::1', 'fd00::/8']) {
			expect(validateAllowedEntry(good)).toBeNull();
		}
	});
});

describe('allowlistCoversIp', () => {
	it('matches exact hosts and IPv4 CIDRs', () => {
		expect(allowlistCoversIp(['192.168.1.100'], '192.168.1.100')).toBe('covered');
		expect(allowlistCoversIp(['10.0.0.0/8'], '10.5.6.7')).toBe('covered');
		expect(allowlistCoversIp(['::1'], '::1')).toBe('covered');
	});

	it('detects exclusion', () => {
		expect(allowlistCoversIp(['192.168.1.100'], '192.168.1.101')).toBe('not-covered');
		expect(allowlistCoversIp(['10.0.0.0/8'], '192.168.1.1')).toBe('not-covered');
		expect(allowlistCoversIp(['not-an-ip'], '192.168.1.1')).toBe('not-covered');
	});

	it('reports unknown when only IPv6 CIDRs could cover', () => {
		expect(allowlistCoversIp(['fd00::/8'], 'fd00::5')).toBe('unknown');
	});

	it('reports unknown for unparsable client IPs', () => {
		expect(allowlistCoversIp(['10.0.0.0/8'], '')).toBe('unknown');
		expect(allowlistCoversIp(['10.0.0.0/8'], 'garbage')).toBe('unknown');
	});
});

describe('decideSave', () => {
	it('blocks invalid entries', () => {
		const decision = decideSave(['10.0.0.0/8', 'not-an-ip'], '10.9.9.9');
		expect(decision).toEqual({ action: 'block', error: 'Invalid IP or CIDR format: not-an-ip' });
	});

	it('saves empty lists (server auto-detect fallback)', () => {
		expect(decideSave([], '10.9.9.9')).toEqual({ action: 'save' });
	});

	it('saves when the client IP stays covered', () => {
		expect(decideSave(['10.0.0.0/8'], '10.9.9.9')).toEqual({ action: 'save' });
		expect(decideSave(['192.168.1.100'], '192.168.1.100')).toEqual({ action: 'save' });
	});

	it('saves when the client IP is unavailable (guard is best-effort)', () => {
		expect(decideSave(['10.9.9.9'], '')).toEqual({ action: 'save' });
	});

	it('asks for confirmation on self-lockout', () => {
		const decision = decideSave(['192.168.99.99'], '10.9.9.9');
		expect(decision.action).toBe('confirm');
		if (decision.action === 'confirm') {
			expect(decision.message).toContain('10.9.9.9');
			expect(decision.message).toContain('revoke your own access');
		}
	});

	it('asks for confirmation when coverage is unverifiable', () => {
		const decision = decideSave(['fd00::/8'], 'fd00::5');
		expect(decision.action).toBe('confirm');
		if (decision.action === 'confirm') {
			expect(decision.message).toContain('Could not verify');
		}
	});
});
