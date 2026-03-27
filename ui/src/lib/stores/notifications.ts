import { writable } from 'svelte/store';

export interface Notification {
	id: number;
	message: string;
	type: 'success' | 'error' | 'info';
}

function createNotificationsStore() {
	const { subscribe, update } = writable<Notification[]>([]);
	let id = 0;

	return {
		subscribe,
		show(message: string, type: Notification['type'] = 'info') {
			const notification: Notification = { id: ++id, message, type };
			update(n => [...n, notification]);
			setTimeout(() => {
				update(n => n.filter(item => item.id !== notification.id));
			}, 5000);
		},
		success(message: string) {
			this.show(message, 'success');
		},
		error(message: string) {
			this.show(message, 'error');
		},
		info(message: string) {
			this.show(message, 'info');
		},
		remove(id: number) {
			update(n => n.filter(item => item.id !== id));
		}
	};
}

export const notifications = createNotificationsStore();
