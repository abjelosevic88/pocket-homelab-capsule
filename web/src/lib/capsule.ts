// Words, icons and dates for capsules.
import type { Capsule, Kind, Section } from './types'
import { fmtDate } from './format'

export const KINDS: { id: Kind; label: string; icon: string; hint: string }[] = [
  { id: 'letter', label: 'Letter', icon: 'feather', hint: 'Write to whoever opens it' },
  { id: 'photo', label: 'Photo', icon: 'image', hint: 'A picture from today' },
  { id: 'audio', label: 'Recording', icon: 'mic', hint: 'Your voice, or the sounds of home' },
  { id: 'video', label: 'Video', icon: 'video', hint: 'A short film' },
  { id: 'prediction', label: 'Prediction', icon: 'sparkles', hint: 'A guess, and how sure you are' },
  { id: 'question', label: 'Question', icon: 'help', hint: 'For whoever opens it to answer' },
  { id: 'goal', label: 'Goal', icon: 'target', hint: 'Something you want to have done by then' },
  { id: 'list', label: 'List', icon: 'list', hint: 'Favourites, prices, a normal day' },
  { id: 'link', label: 'Link', icon: 'link', hint: 'A page worth keeping' },
  { id: 'file', label: 'File', icon: 'paperclip', hint: 'A document or anything else' },
]

export const kindOf = (k: Kind) => KINDS.find(x => x.id === k) || KINDS[0]

export function kindWords(k: Kind, n: number): string {
  const one: Record<Kind, [string, string]> = {
    letter: ['letter', 'letters'], photo: ['photo', 'photos'], audio: ['recording', 'recordings'], video: ['video', 'videos'], file: ['file', 'files'],
    prediction: ['prediction', 'predictions'], question: ['question', 'questions'], goal: ['goal', 'goals'], list: ['list', 'lists'], link: ['link', 'links'],
  }
  return `${n} ${n === 1 ? one[k][0] : one[k][1]}`
}

export const SECTION_ICON: Record<Section, string> = { weather: 'cloud', reading: 'book-open', listening: 'music', places: 'globe', health: 'footprints', wealth: 'coins' }

/** When it opens, in words: "Fri 20 Nov 2026, 09:00". */
export function opensWords(c: Capsule): string {
  const d = new Date(c.opens_at)
  const day = fmtDate(localISO(d))
  const hm = d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' })
  return hm === '00:00' ? day : `${day}, ${hm}`
}

export function localISO(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

export function dayOf(iso: string | null | undefined): string {
  return iso ? fmtDate(localISO(new Date(iso))) : ''
}

/** The parts of the wait: years, months, days, hours, minutes. */
export function countdown(to: string, from = new Date()): { y: number; mo: number; d: number; h: number; mi: number; past: boolean } {
  const end = new Date(to)
  if (end <= from) return { y: 0, mo: 0, d: 0, h: 0, mi: 0, past: true }
  let y = end.getFullYear() - from.getFullYear()
  let mo = end.getMonth() - from.getMonth()
  let d = end.getDate() - from.getDate()
  let h = end.getHours() - from.getHours()
  let mi = end.getMinutes() - from.getMinutes()
  if (mi < 0) { mi += 60; h-- }
  if (h < 0) { h += 24; d-- }
  if (d < 0) { d += new Date(end.getFullYear(), end.getMonth(), 0).getDate(); mo-- }
  if (mo < 0) { mo += 12; y-- }
  return { y, mo, d, h, mi, past: false }
}

/** How long it waited (sealed → opened), in words. */
export function waited(c: Capsule): string {
  if (!c.sealed_at) return ''
  const a = new Date(c.sealed_at), b = c.opened_at ? new Date(c.opened_at) : new Date()
  const cd = countdown(b.toISOString(), a)
  if (cd.y > 0) return `${cd.y} ${cd.y === 1 ? 'year' : 'years'}${cd.mo ? `, ${cd.mo} ${cd.mo === 1 ? 'month' : 'months'}` : ''}`
  if (cd.mo > 0) return `${cd.mo} ${cd.mo === 1 ? 'month' : 'months'}`
  return `${cd.d} ${cd.d === 1 ? 'day' : 'days'}`
}

export const stateWords = (c: Capsule, ready: boolean) => ready ? 'Ready to open' : c.state === 'filling' ? 'Being filled' : c.state === 'sealed' ? 'Sealed' : c.early ? 'Opened early' : 'Opened'

export const uploadURL = (id: number, thumb = false) => `/api/v1/uploads/${id}${thumb ? '?thumb=1' : ''}`

export const OUTCOMES: { id: 'yes' | 'partly' | 'no'; label: string }[] = [{ id: 'yes', label: 'Came true' }, { id: 'partly', label: 'Partly' }, { id: 'no', label: 'No' }]
export const GOAL_OUTCOMES: { id: 'yes' | 'partly' | 'no'; label: string }[] = [{ id: 'yes', label: 'Done' }, { id: 'partly', label: 'Partly' }, { id: 'no', label: 'Not done' }]
