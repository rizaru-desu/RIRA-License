// TODO: replace with real notifications API — dummy data

export type DummyNotificationKind = 'warning' | 'activation' | 'signin' | 'security' | 'summary';

export interface DummyNotification {
	id: string;
	text: string;
	time: string;
	kind: DummyNotificationKind;
	unread: boolean;
}

export const INITIAL_DUMMY_NOTIFICATIONS: DummyNotification[] = [
	{
		id: 'notif-1',
		text: 'License "PRO-2026-0142" expires in 7 days',
		time: '15m ago',
		kind: 'warning',
		unread: true
	},
	{
		id: 'notif-2',
		text: 'New activation from a new device',
		time: '1h ago',
		kind: 'activation',
		unread: true
	},
	{
		id: 'notif-3',
		text: 'New sign-in from Chrome on Windows',
		time: '3h ago',
		kind: 'signin',
		unread: true
	},
	{
		id: 'notif-4',
		text: 'Backup codes regenerated',
		time: '1d ago',
		kind: 'security',
		unread: false
	},
	{
		id: 'notif-5',
		text: 'Weekly summary: 12 active licenses',
		time: '3d ago',
		kind: 'summary',
		unread: false
	}
];

