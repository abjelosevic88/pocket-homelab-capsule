// Pieces every capsule page uses: its cover, the countdown, one item, the
// world of the day, and buttons that need another Pocket app.
import React, { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import type { AppSlug, Capsule, CapsuleRow, Item, Snapshot } from '../lib/types'
import { countdown, dayOf, kindOf, opensWords, SECTION_ICON, stateWords, uploadURL } from '../lib/capsule'
import { useAppState } from '../lib/data'
import { useApp } from '../lib/app-context'
import { api, V1 } from '../lib/api'
import { Icon } from './icons'

export function capsuleStyle(c: Capsule): React.CSSProperties {
  return c.color ? ({ '--cap': c.color } as React.CSSProperties) : {}
}

/** The capsule as a card in a list. */
export function CapsuleCard({ row }: { row: CapsuleRow }) {
  const c = row.capsule
  const sealed = c.state === 'sealed'
  return (
    <Link to={`/c/${c.id}`} className={`cap-card st-${row.ready ? 'ready' : c.state}`} style={capsuleStyle(c)}>
      <div className="cap-card-top">
        <span className="cap-emoji" aria-hidden>{c.emoji}</span>
        <span className={`cap-state st-${row.ready ? 'ready' : c.state}`}>
          <Icon name={row.ready ? 'gift' : sealed ? 'lock' : c.state === 'filling' ? 'feather' : 'unlock'} size={12} strokeWidth={2.2} />{stateWords(c, row.ready)}
        </span>
      </div>
      <div className="cap-card-title">{c.title}</div>
      <div className="cap-card-meta">
        {c.state === 'opened' ? <>Opened {dayOf(c.opened_at)}</> : row.ready ? <>Its day was {dayOf(c.opens_at)}</> : <>Opens {opensWords(c)}</>}
      </div>
      <div className="cap-card-foot">
        <span>{c.items} {c.items === 1 ? 'thing' : 'things'} inside</span>
        {c.state !== 'opened' && !row.ready && row.opens_in && <span className="cap-wait">{row.opens_in}</span>}
        {row.role !== 'owner' && <span className="muted">by {c.owner_name}</span>}
      </div>
    </Link>
  )
}

/** The big countdown: years · months · days · hours, ticking each minute. */
export function Countdown({ to, compact = false }: { to: string; compact?: boolean }) {
  const [, tick] = useState(0)
  useEffect(() => { const t = window.setInterval(() => tick(n => n + 1), 30000); return () => window.clearInterval(t) }, [])
  const cd = countdown(to)
  if (cd.past) return null
  const parts: [number, string][] = cd.y > 0 ? [[cd.y, cd.y === 1 ? 'year' : 'years'], [cd.mo, cd.mo === 1 ? 'month' : 'months'], [cd.d, cd.d === 1 ? 'day' : 'days']]
    : cd.mo > 0 ? [[cd.mo, cd.mo === 1 ? 'month' : 'months'], [cd.d, cd.d === 1 ? 'day' : 'days'], [cd.h, cd.h === 1 ? 'hour' : 'hours']]
      : [[cd.d, cd.d === 1 ? 'day' : 'days'], [cd.h, cd.h === 1 ? 'hour' : 'hours'], [cd.mi, cd.mi === 1 ? 'minute' : 'minutes']]
  return (
    <div className={`countdown ${compact ? 'compact' : ''}`} aria-label={parts.map(p => `${p[0]} ${p[1]}`).join(', ')}>
      {parts.map(([n, w]) => <div key={w} className="cd-part"><span className="cd-n">{n}</span><span className="cd-w">{w}</span></div>)}
    </div>
  )
}

/** Plain text with paragraphs (letters are not Markdown: what was written is what shows). */
export function Prose({ text }: { text: string }) {
  return <div className="prose">{text.split(/\n{2,}/).map((p, i) => <p key={i}>{p.split('\n').map((l, j) => <React.Fragment key={j}>{j > 0 && <br />}{l}</React.Fragment>)}</p>)}</div>
}

/** One thing in a capsule. `upload` makes the URL of its file (public pages use their own). */
export function ItemCard({ it, upload = uploadURL, actions, children }: { it: Item; upload?: (id: number, thumb?: boolean) => string; actions?: React.ReactNode; children?: React.ReactNode }) {
  const k = kindOf(it.kind)
  return (
    <article className={`item k-${it.kind}`} id={`item-${it.id}`}>
      <header className="item-head">
        <span className="item-ico"><Icon name={k.icon} size={15} /></span>
        <span className="item-kind">{k.label}</span>
        <span className="item-by">{it.author_name}{it.guest && <span className="muted"> · without an account</span>}{it.after_seal && <span className="muted"> · through the slot</span>}</span>
        <span className="spacer" />
        {it.from_app && <span className="item-from" title={`From Pocket ${cap(it.from_app)}`}>{it.from_url ? <a href={it.from_url} target="_blank" rel="noreferrer">from {cap(it.from_app)}</a> : <>from {cap(it.from_app)}</>}</span>}
        {actions}
      </header>
      {it.title && it.kind !== 'link' && <h3 className="item-title">{it.title}</h3>}
      <ItemBody it={it} upload={upload} />
      {children}
    </article>
  )
}

const cap = (s: string) => s.charAt(0).toUpperCase() + s.slice(1)

function ItemBody({ it, upload }: { it: Item; upload: (id: number, thumb?: boolean) => string }) {
  const u = it.upload_id
  switch (it.kind) {
    case 'letter':
      return <Prose text={it.body} />
    case 'photo':
      return <>{u && <a href={upload(u)} target="_blank" rel="noreferrer" className="item-photo"><img src={upload(u, true)} alt={it.title || 'photo'} loading="lazy" /></a>}{it.body && <p className="item-caption">{it.body}</p>}</>
    case 'audio':
      return <>{u && <audio controls preload="none" src={upload(u)} className="item-audio" />}{it.body && <details className="item-transcript"><summary>What was said</summary><Prose text={it.body} /></details>}</>
    case 'video':
      return <>{u && <video controls preload="metadata" src={upload(u)} className="item-video" />}{it.body && <p className="item-caption">{it.body}</p>}</>
    case 'file':
      return <>{u && <a href={upload(u)} className="item-file" download={it.file_name}><Icon name="paperclip" size={15} />{it.file_name || 'the file'}{it.bytes ? <span className="muted"> · {Math.round(it.bytes / 1024)} KB</span> : null}</a>}{it.body && <p className="item-caption">{it.body}</p>}</>
    case 'prediction':
      return <div className="item-guess"><p className="item-big">{it.body}</p><div className="guess-sure"><span className="guess-bar"><i style={{ width: `${it.confidence}%` }} /></span>{it.confidence}% sure</div></div>
    case 'question':
    case 'goal':
      return <p className="item-big">{it.body}</p>
    case 'list':
      return <dl className="item-list">{it.entries.map((e, i) => <div key={i}><dt>{e.label}</dt><dd>{e.value}</dd></div>)}</dl>
    case 'link':
      return <><a href={it.url} target="_blank" rel="noreferrer" className="item-link"><Icon name="external" size={14} />{it.title || it.url}</a>{it.body && <p className="item-caption">{it.body}</p>}</>
  }
  return null
}

/** The world of a day, as rows. */
export function SnapshotView({ snap, other, labels = ['Then', 'Now'] }: { snap: Snapshot; other?: Snapshot | null; labels?: [string, string] }) {
  const rows: { id: keyof typeof SECTION_ICON; label: string; a: string; b?: string }[] = []
  const line = (s: Snapshot | null | undefined, id: string): string => {
    if (!s) return ''
    switch (id) {
      case 'weather': return s.weather ? `${s.weather.emoji} ${s.weather.temp !== undefined ? Math.round(s.weather.temp) + ' °C, ' : ''}${s.weather.condition || s.weather.text} in ${s.weather.place}` : ''
      case 'reading': return s.reading?.length ? s.reading.map(b => b.author ? `${b.title} by ${b.author}` : b.title).join('; ') : ''
      case 'listening': return s.listening?.artists?.length ? s.listening.artists.map(a => a.name).join(', ') : ''
      case 'places': return s.places ? `${s.places.countries.length} ${s.places.countries.length === 1 ? 'country' : 'countries'} in ${s.places.year}${s.places.countries.length ? ': ' + s.places.countries.join(', ') : ''}${s.places.top?.length ? ` · most: ${s.places.top.slice(0, 3).join(', ')}` : ''}` : ''
      case 'health': return s.health ? [s.health.steps ? `${Math.round(s.health.steps).toLocaleString()} steps` : '', s.health.sleep ? `slept ${s.health.sleep}` : '', s.health.weight || ''].filter(Boolean).join(' · ') : ''
      case 'wealth': return s.wealth ? `${Math.round(s.wealth.net / 100).toLocaleString()} ${s.wealth.currency}` : ''
    }
    return ''
  }
  const names: Record<string, string> = { weather: 'Weather', reading: 'Reading', listening: 'Listening to', places: 'Your year', health: 'Health', wealth: 'Net worth' }
  for (const id of Object.keys(SECTION_ICON) as (keyof typeof SECTION_ICON)[]) {
    const a = line(snap, id), b = other ? line(other, id) : undefined
    if (a || b) rows.push({ id, label: names[id], a, b })
  }
  if (!rows.length) return <p className="muted small">Nothing from your other apps was kept.</p>
  return (
    <div className={`snap ${other !== undefined ? 'two' : ''}`}>
      {other !== undefined && <div className="snap-row snap-headrow"><span /><span className="snap-col-h">{labels[0]}</span><span className="snap-col-h">{labels[1]}</span></div>}
      {rows.map(r => (
        <div key={r.id} className="snap-row">
          <span className="snap-label"><Icon name={SECTION_ICON[r.id]} size={15} />{r.label}</span>
          <span className="snap-val">{r.a || <span className="faint">—</span>}</span>
          {other !== undefined && <span className="snap-val now">{r.b || <span className="faint">—</span>}</span>}
        </div>
      ))}
    </div>
  )
}

/** Is an AI there to help (Pocket AI or the person's own key)? Asked once per page. */
let aiCache: Promise<{ ready: boolean; detail: string }> | null = null
export function useAIReady() {
  const { settings } = useApp()
  const [st, setSt] = useState<{ ready: boolean; detail: string } | null>(null)
  useEffect(() => {
    if (!settings?.ai_help) return
    aiCache ||= api.get<{ ready: boolean; detail: string }>(`${V1}/ai/status`).catch(() => ({ ready: false, detail: 'the AI did not answer' }))
    aiCache.then(setSt)
    const t = window.setTimeout(() => { aiCache = null }, 60000)
    return () => window.clearTimeout(t)
  }, [settings?.ai_help])
  const on = !!settings?.ai_help
  return { on, ready: on && !!st?.ready, why: !on ? 'AI help is off (Settings → AI)' : st && !st.ready ? `No AI yet: ${st.detail || 'set it up in Settings → AI'}` : '' }
}

/** Is this Pocket app (and scope) reachable for me? With why not. */
export function useSibling(slug: AppSlug, scope?: string) {
  const st = useAppState()
  const { settings } = useApp()
  const r = st(slug, scope)
  const coreOn = !!settings?.core_configured && settings?.core_registered !== false
  const why = r.ok ? '' : r.app ? (r.app.fix ? `${r.app.detail ? r.app.detail + '. ' : ''}${r.app.fix}` : r.app.detail) || `${r.app.name} is not connected` : coreOn ? 'Checking…' : 'Needs Pocket Core or a direct connection (Settings → Connected apps)'
  return { ok: r.ok, loading: r.loading, why, name: r.app?.name || slug }
}

/** A button that needs another Pocket app: greyed with the reason when it is not there. */
export function AppButton({ slug, scope, icon, children, onClick, busy, className = 'btn' }: { slug: AppSlug; scope?: string; icon: string; children: React.ReactNode; onClick: () => void; busy?: boolean; className?: string }) {
  const s = useSibling(slug, scope)
  return (
    <button type="button" className={className} disabled={!s.ok || busy} onClick={onClick} title={s.ok ? undefined : s.why}>
      <Icon name={icon} size={15} />{children}
    </button>
  )
}
