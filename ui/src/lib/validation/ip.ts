/**
 * Client-side parsing/validation for `allowed_ips` entries.
 *
 * Mirrors the backend `ParseAllowedIPEntry` semantics: each entry is either
 * CIDR notation ("192.168.1.0/24") or a bare IP address ("192.168.1.100",
 * "::1"). The backend remains the source of truth — this module exists so
 * the settings form can flag obvious typos before saving (a mistyped list
 * now fails closed server-side and could lock the admin out).
 */

export interface ParsedEntry {
	/** Normalized form, e.g. "192.168.1.100/32" or "192.168.1.0/24". */
	normalized: string;
	kind: 'cidr' | 'host';
	family: 4 | 6;
	/** IPv4 network address as uint32 (family 4 only). */
	network?: number;
	/** Prefix length (family 4 only; always set for family 4). */
	prefix?: number;
}

function parseIPv4(text: string): number | null {
	const parts = text.split('.');
	if (parts.length !== 4) return null;
	let value = 0;
	for (const part of parts) {
		if (!/^\d{1,3}$/.test(part)) return null;
		const n = Number(part);
		if (n < 0 || n > 255) return null;
		value = value * 256 + n;
	}
	return value >>> 0;
}

function normalizeIPv6Group(group: string): string | null {
	if (!/^[0-9a-fA-F]{1,4}$/.test(group)) return null;
	return Number.parseInt(group, 16).toString(16);
}

function normalizeIPv6(text: string): string | null {
	const withoutZone = text.split('%')[0];
	if (!/^[0-9a-fA-F:.]+$/.test(withoutZone) || !withoutZone.includes(':')) return null;
	const halves = withoutZone.split('::');
	if (halves.length > 2) return null;
	const expand = (side: string): string[] | null => {
		if (side === '') return [];
		const groups = side.split(':');
		if (groups.length > 8) return null;
		const out: string[] = [];
		for (const g of groups) {
			// Embedded IPv4 tail (e.g. ::ffff:192.168.1.1) — accept, backend decides.
			if (g.includes('.')) {
				if (parseIPv4(g) === null) return null;
				out.push(g);
				continue;
			}
			const n = normalizeIPv6Group(g);
			if (n === null) return null;
			out.push(n);
		}
		return out;
	};
	const left = expand(halves[0]);
	const right = halves.length === 2 ? expand(halves[1]) : [];
	if (left === null || right === null) return null;
	if (halves.length === 1 && left.length !== 8) return null;
	if (halves.length === 2 && left.length + right.length > 7) return null;
	if (halves.length === 2) return `${left.join(':')}::${right.join(':')}`;
	return left.join(':');
}

/**
 * Parse a single allowlist entry. Returns the parsed form, or null when the
 * entry is invalid. Bare IPs normalize to /32 (IPv4) or /128 (IPv6).
 */
export function parseAllowedEntry(entry: string): ParsedEntry | null {
	const trimmed = entry.trim();
	if (trimmed === '') return null;

	const slash = trimmed.lastIndexOf('/');
	if (slash !== -1) {
		const addrText = trimmed.slice(0, slash);
		const maskText = trimmed.slice(slash + 1);
		if (!/^\d{1,3}$/.test(maskText)) return null;
		const mask = Number(maskText);

		const v4 = parseIPv4(addrText);
		if (v4 !== null) {
			if (mask < 0 || mask > 32) return null;
			return {
				normalized: `${addrText}/${mask}`,
				kind: 'cidr',
				family: 4,
				network: v4,
				prefix: mask
			};
		}
		const v6 = normalizeIPv6(addrText);
		if (v6 !== null) {
			if (mask < 0 || mask > 128) return null;
			return { normalized: `${v6}/${mask}`, kind: 'cidr', family: 6 };
		}
		// Zoned CIDR the backend would accept (fe80::1%eth0/64): strip the
		// zone and re-check so the form doesn't reject it.
		const zoned = addrText.split('%')[0];
		if (zoned !== addrText) {
			const retry = parseAllowedEntry(`${zoned}/${maskText}`);
			if (retry !== null) return retry;
		}
		return null;
	}

	const v4 = parseIPv4(trimmed);
	if (v4 !== null) {
		const dotted = trimmed;
		return { normalized: `${dotted}/32`, kind: 'host', family: 4, network: v4, prefix: 32 };
	}
	const v6 = normalizeIPv6(trimmed);
	if (v6 !== null) {
		return { normalized: `${v6}/128`, kind: 'host', family: 6 };
	}
	return null;
}

/** Validate one entry; returns an error message, or null when valid. */
export function validateAllowedEntry(entry: string): string | null {
	if (entry.trim() === '') return 'entry must not be empty';
	if (parseAllowedEntry(entry) === null) return `invalid IP or CIDR format: ${entry.trim()}`;
	return null;
}

export type Coverage = 'covered' | 'not-covered' | 'unknown';

/**
 * Determine whether `clientIp` is covered by `entries`.
 *
 * - Exact host matches (v4 and v6) and IPv4 CIDR containment are computed.
 * - IPv6 CIDR containment cannot be computed here: if nothing else covers
 *   the client IP but such entries exist, the result is 'unknown' (the
 *   backend decides).
 * - A list with no valid entries denies everything server-side, so the
 *   result is 'not-covered'.
 */
export function allowlistCoversIp(entries: string[], clientIp: string): Coverage {
	const clientText = clientIp.trim().split('%')[0];
	if (clientText === '') return 'unknown';

	const clientV4 = parseIPv4(clientText);
	const clientV6 = clientV4 === null ? normalizeIPv6(clientText) : null;
	if (clientV4 === null && clientV6 === null) return 'unknown';

	let anyValid = false;
	let hasUncomputable = false;

	for (const entry of entries) {
		const parsed = parseAllowedEntry(entry);
		if (parsed === null) continue;
		anyValid = true;

		if (parsed.kind === 'host') {
			const hostText = parsed.normalized.split('/')[0];
			if (parsed.family === 4 && clientV4 !== null) {
				if (parseIPv4(hostText) === clientV4) return 'covered';
			} else if (parsed.family === 6 && clientV6 !== null) {
				if (normalizeIPv6(hostText) === clientV6) return 'covered';
			}
			continue;
		}

		if (
			parsed.family === 4 &&
			clientV4 !== null &&
			parsed.network !== undefined &&
			parsed.prefix !== undefined
		) {
			const mask = parsed.prefix === 0 ? 0 : (0xffffffff << (32 - parsed.prefix)) >>> 0;
			if ((clientV4 & mask) >>> 0 === (parsed.network & mask) >>> 0) return 'covered';
		} else {
			hasUncomputable = true;
		}
	}

	if (!anyValid) return 'not-covered';
	return hasUncomputable ? 'unknown' : 'not-covered';
}

export type SaveDecision =
	| { action: 'save' }
	| { action: 'confirm'; message: string }
	| { action: 'block'; error: string };

/**
 * Decide what the settings form should do when saving `ips` for the admin
 * at `clientIp`. Pure logic extracted for unit testing: 'block' on invalid
 * entries, 'confirm' when the admin's own access is at risk, 'save'
 * otherwise (empty lists fall back to server auto-detect; an unavailable
 * client IP skips the guard rather than blocking the save).
 */
export function decideSave(ips: string[], clientIp: string): SaveDecision {
	const invalid = ips.filter((ip) => validateAllowedEntry(ip) !== null);
	if (invalid.length > 0) {
		return { action: 'block', error: `Invalid IP or CIDR format: ${invalid.join(', ')}` };
	}
	if (ips.length === 0 || clientIp === '') {
		return { action: 'save' };
	}
	const coverage = allowlistCoversIp(ips, clientIp);
	if (coverage === 'not-covered') {
		return {
			action: 'confirm',
			message: `Your current IP (${clientIp}) is not covered by this allowlist. Saving will revoke your own access. Save anyway?`
		};
	}
	if (coverage === 'unknown') {
		return {
			action: 'confirm',
			message: `Could not verify that your current IP (${clientIp}) is covered by this allowlist. Save anyway?`
		};
	}
	return { action: 'save' };
}
