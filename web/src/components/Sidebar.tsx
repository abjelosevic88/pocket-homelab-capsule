// Left column: a new capsule, the sections with what waits in each, and
// the capsules that open next.
import { useCallback, useEffect, useState } from 'react'
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useOnChange } from '../lib/data'
import type { CapsuleRow, Home } from '../lib/types'
import { Icon } from './icons'

export function Sidebar({ onCreate, onNavigate }: { onCreate: () => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [home, setHome] = useState<Home | null>(null)
  const [rows, setRows] = useState<CapsuleRow[]>([])
  const load = useCallback(() => {
    api.get<Home>(`${V1}/home`).then(setHome).catch(() => {})
    api.get<{ capsules: CapsuleRow[] }>(`${V1}/capsules`).then(r => setRows(r.capsules.filter(c => c.capsule.state !== 'opened' || c.ready))).catch(() => {})
  }, [])
  useEffect(() => { load() }, [load, loc.pathname])
  useOnChange(load)

  const title = settings?.title || status?.title || 'Pocket Capsule'
  const path = loc.pathname
  const show = new URLSearchParams(loc.search).get('show')
  const c = home?.counts
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number; hint?: string; strong?: boolean }[] = [
    { to: '/', label: 'All capsules', icon: 'hourglass', active: path === '/' && !show },
    { to: '/?show=ready', label: 'Ready to open', icon: 'gift', active: path === '/' && show === 'ready', count: c?.ready || undefined, hint: 'Their day has come', strong: true },
    { to: '/?show=filling', label: 'Being filled', icon: 'feather', active: path === '/' && show === 'filling', count: c?.filling || undefined, hint: 'Not sealed yet' },
    { to: '/?show=sealed', label: 'Sealed', icon: 'lock', active: path === '/' && show === 'sealed', count: c?.sealed || undefined, hint: 'Waiting for their day' },
    { to: '/?show=opened', label: 'Opened', icon: 'unlock', active: path === '/' && show === 'opened', count: c?.opened || undefined },
    { to: '/predictions', label: 'Predictions', icon: 'sparkles', active: path.startsWith('/predictions'), hint: 'How your guesses turned out' },
    { to: '/yearly', label: 'Yearly capsules', icon: 'repeat', active: path.startsWith('/yearly'), hint: 'A nudge each year to write the next one' },
  ]
  const soon = rows.filter(r => r.capsule.state !== 'opened').slice(0, 6)
  return (
    <aside className="sidebar" aria-label="Pocket Capsule">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Capsule' ? title : <>Pocket <b>Capsule</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <button type="button" className="btn primary create-btn" onClick={onCreate} title="New capsule (N)">
        <Icon name="plus" size={15} strokeWidth={2.2} />New capsule
      </button>

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(n => (
          <NavLink key={n.to} to={n.to} onClick={onNavigate} title={n.hint} className={() => `sb-nav-item ${n.active ? 'active' : ''}`}>
            <Icon name={n.icon} size={16} />{n.label}
            {!!n.count && <span className={`sb-count ${n.strong ? 'strong' : ''}`} title={n.hint}>{n.count}</span>}
          </NavLink>
        ))}
      </nav>

      <section className="sb-sec">
        <h4 className="sb-h"><Link to="/" onClick={onNavigate} className="sb-h-link">Opening next</Link></h4>
        {soon.map(r => (
          <button key={r.capsule.id} type="button" className={`sb-row sb-list ${path === `/c/${r.capsule.id}` ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/c/${r.capsule.id}`) }} title={r.capsule.title}>
            <span className="sb-emoji" aria-hidden>{r.capsule.emoji}</span>
            <span className="grow ellipsis">{r.capsule.title}</span>
            {r.ready ? <span className="sb-count strong" title="Ready to open">open</span> : <span className="muted small nowrap">{r.capsule.state === 'filling' ? 'filling' : shortWait(r.opens_in)}</span>}
          </button>
        ))}
        {home && soon.length === 0 && <button type="button" className="sb-row sb-list" onClick={() => { onNavigate(); onCreate() }}><Icon name="plus" size={15} className="sb-list-ico" /><span className="grow">Make your first capsule</span></button>}
      </section>

      <div className="sb-foot">
        <Link to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</Link>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('pcsl:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}

/** "in 2 years, 3 months" → "2 y" for the narrow column. */
function shortWait(s: string): string {
  const m = /in (\d+) (year|month|day|hour|minute)/.exec(s)
  if (!m) return s.replace(/^in /, '')
  return m[1] + ' ' + ({ year: 'y', month: 'mo', day: 'd', hour: 'h', minute: 'min' } as Record<string, string>)[m[2]]
}
