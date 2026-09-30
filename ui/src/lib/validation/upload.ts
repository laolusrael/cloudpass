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
