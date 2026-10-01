/**
 * Client-side validation/formatting for the instance file-upload form.
 *
 * The backend remains the source of truth (it enforces the size cap on the
 * streamed bytes and resolves the target path) — this module exists so the
 * upload modal can flag obvious problems before sending.
 */

/** Returns an error message when the file cannot be uploaded, else null. */
export function validateSelectedFile(file: File, maxSizeMB: number): string | null {
	if (file.size === 0) {
		return 'The selected file is empty.';
	}
	if (file.size > maxSizeMB * 1024 * 1024) {
		return `The selected file exceeds the maximum of ${maxSizeMB} MB.`;
	}
	return null;
}

export function formatFileSize(bytes: number): string {
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
	return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

/**
 * Validate the Settings upload fields before saving. Mirrors the backend
 * `ValidateUploadSettings` rules; the server remains authoritative.
 * Returns an error message, or null when valid.
 */
export function validateUploadSettings(
	maxSizeMB: unknown,
	defaultPath: string,
	stagingDir = ''
): string | null {
	if (
		typeof maxSizeMB !== 'number' ||
		!Number.isInteger(maxSizeMB) ||
		maxSizeMB < 1 ||
		maxSizeMB > 1024
	) {
		return 'Max file size must be a whole number between 1 and 1024 MB.';
	}
	if (!defaultPath.startsWith('/')) {
		return 'Default upload path must be an absolute path.';
	}
	if (defaultPath.split('/').includes('..')) {
		return 'Default upload path must not contain ..';
	}
	if (stagingDir !== '' && !(stagingDir.startsWith('/') || /^[A-Za-z]:[\\/]/.test(stagingDir))) {
		return 'Staging directory must be an absolute path.';
	}
	return null;
}

/**
 * Validate the Settings image-size field. Mirrors backend ValidateImageSizeMB.
 */
export function validateImageSizeMB(maxImageSizeMB: unknown): string | null {
	if (
		typeof maxImageSizeMB !== 'number' ||
		!Number.isInteger(maxImageSizeMB) ||
		maxImageSizeMB < 1 ||
		maxImageSizeMB > 102400
	) {
		return 'Max image size must be a whole number between 1 and 102400 MB.';
	}
	return null;
}
