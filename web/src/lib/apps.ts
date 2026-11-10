// The other Pocket apps Capsule works with.
import type { AppSlug, AppStateName } from './types'

export const APP_INFO: Record<AppSlug, { name: string; icon: string }> = {
  calendar: { name: 'Pocket Calendar', icon: 'calendar' }, contacts: { name: 'Pocket Contacts', icon: 'users' }, family: { name: 'Pocket Family', icon: 'home' },
  email: { name: 'Pocket Email', icon: 'mail' }, chat: { name: 'Pocket Chat', icon: 'message' }, photos: { name: 'Pocket Photos', icon: 'image' },
  notes: { name: 'Pocket Notes', icon: 'note' }, files: { name: 'Pocket Files', icon: 'archive' }, voice: { name: 'Pocket Voice', icon: 'mic' },
  tasks: { name: 'Pocket Tasks', icon: 'check-square' }, alerts: { name: 'Pocket Alerts', icon: 'bell' }, blog: { name: 'Pocket Blog', icon: 'feather' },
  weather: { name: 'Pocket Weather', icon: 'cloud' }, bookshelf: { name: 'Pocket Bookshelf', icon: 'book-open' }, music: { name: 'Pocket Music', icon: 'music' },
  movement: { name: 'Pocket Movement', icon: 'globe' }, health: { name: 'Pocket Health', icon: 'footprints' }, wealth: { name: 'Pocket Wealth', icon: 'coins' },
}

export const STATE_WORDS: Record<AppStateName, string> = {
  on: 'Connected', partly: 'Partly connected', off: 'Off', link_off: 'Link is off in Pocket Core', missing: 'Not on this box', old: 'Needs an update',
  no_core: 'No Pocket Core', not_set_up: 'Not set up', error: 'Not working',
}
