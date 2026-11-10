// The shapes the API sends.

export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = 'core' | 'direct' | ''
export type AppSlug = 'calendar' | 'contacts' | 'family' | 'email' | 'chat' | 'photos' | 'notes' | 'files' | 'voice' | 'tasks' | 'alerts' | 'blog'
  | 'weather' | 'bookshelf' | 'music' | 'movement' | 'health' | 'wealth'
type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  open_hour: number; remind_days: number; sections: Section[]; surprise: boolean; slot: boolean
  calendar_on_seal: boolean; alerts_on_ready: boolean; notes_folder: string; files_folder: string; chat_to: string; ai_help: boolean
  notify_ready: boolean; notify_remind: boolean; notify_added: boolean; notify_opened: boolean; notify_yearly: boolean; notify_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; demo: boolean
}

export type AppStateName = 'on' | 'partly' | 'off' | 'link_off' | 'missing' | 'old' | 'no_core' | 'not_set_up' | 'error'
export interface AppState { slug: AppSlug; name: string; reason: string; mode: ConnMode; via: ConnVia; state: AppStateName; detail: string; fix: string; on: string[]; open_url: string }

export type State = 'filling' | 'sealed' | 'opened'
export type Kind = 'letter' | 'photo' | 'audio' | 'video' | 'file' | 'prediction' | 'question' | 'goal' | 'list' | 'link'
export type Role = 'owner' | 'contributor' | 'viewer'
export type Section = 'weather' | 'reading' | 'listening' | 'places' | 'health' | 'wealth'

export interface Capsule {
  id: number; owner_id: number; owner_name: string; title: string; about: string; template: string; emoji: string; color: string; state: State
  opens_at: string; open_rule: 'date' | 'birthday' | 'anniversary'; for_name: string; for_date: string; for_age: number
  surprise: boolean; slot: boolean; deliver: 'on_open' | 'on_date'; remind_days: number; sections: Section[]; snapshot?: Snapshot
  sealed_at: string | null; opened_at: string | null; opened_by: number | null; early: boolean; early_reason: string; delivered_at: string | null
  reflection: string; then_now: string; reply_to: number | null; give_open: boolean; calendar_url: string; source: string
  created_at: string; updated_at: string; items: number; members_n: number
}

export interface Entry { label: string; value: string }

export interface Item {
  id: number; capsule_id: number; user_id: number | null; author_name: string; kind: Kind; title: string; body: string; upload_id: number | null
  confidence: number; outcome: '' | 'yes' | 'partly' | 'no'; answer: string; entries: Entry[]; url: string; from_app: string; from_url: string
  after_seal: boolean; guest: boolean; position: number; source: string; created_at: string; updated_at: string
  mime?: string; file_name?: string; bytes?: number
}

export interface Member { capsule_id: number; user_id: number; name: string; email: string; role: 'contributor' | 'viewer'; added_at: string }
export interface Recipient { id: number; name: string; email: string; sent_at: string | null; viewed_at: string | null; url: string }

export interface Template {
  id: string; name: string; emoji: string; about: string; open_rule: 'date' | 'birthday' | 'anniversary' | 'newyear'; years: number; together: boolean
  prompts: string[]; questions: string[]; predictions: string[]; lists: { title: string; labels: string[] }[]
}

export interface CapsuleRow { capsule: Capsule; role: Role; ready: boolean; opens_in: string; url: string }

export interface CapsuleView extends CapsuleRow {
  items: Item[]; hidden: number; kinds: { kind: Kind; n: number }[]; from: { name: string; n: number }[]
  members: Member[]; recipients: Recipient[]; give_url?: string; can_add: boolean; why_not_add: string; can_open: boolean; why_not_open: string; template: Template
}

export interface Snapshot {
  taken_at?: string
  weather?: { place: string; condition: string; emoji: string; text: string; temp?: number; t_max?: number; t_min?: number }
  reading?: { title: string; author: string }[]
  listening?: { artists: { name: string; plays: number }[]; songs: { name: string; artist: string; plays: number }[]; plays: number }
  places?: { year: number; countries: string[]; places: number; cities: number; top: string[] }
  health?: { steps?: number; sleep?: string; weight?: string; resting_hr?: number }
  wealth?: { net: number; currency: string }
  missing?: Record<string, string>
}

export interface SectionInfo { id: Section; name: string; app: string; slug: AppSlug; scope: string }

export interface HistoryEvent { id: number; capsule_id: number; user_id: number | null; who: string; what: string; detail: string; at: string; title?: string; emoji?: string }

export interface Ritual { id: number; title: string; template: string; month: number; day: number; years: number; active: boolean; last_year: number; created_at: string }

export interface Home {
  counts: { ready: number; filling: number; sealed: number; opened: number }; total: number
  next?: { capsule: Capsule; opens_in: string }; yearly_due: Ritual[]
}
