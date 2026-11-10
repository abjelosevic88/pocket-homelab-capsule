// All capsules: what is ready to open, what opens next, what is being
// filled, and the ones already opened.
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useOnChange } from '../lib/data'
import type { CapsuleRow, Home as HomeT, Template } from '../lib/types'
import { CapsuleCard, Countdown, capsuleStyle } from '../components/capsule'
import { opensWords } from '../lib/capsule'
import { Empty, Loading, Segmented, errMsg } from '../components/ui'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'

type Show = 'all' | 'ready' | 'filling' | 'sealed' | 'opened'

export default function Home() {
  const [params, setParams] = useSearchParams()
  const show = (params.get('show') as Show) || 'all'
  const navigate = useNavigate()
  const [rows, setRows] = useState<CapsuleRow[] | null>(null)
  const [home, setHome] = useState<HomeT | null>(null)
  const [templates, setTemplates] = useState<Template[]>([])
  const [error, setError] = useState('')
  useCrumb(undefined)
  const load = useCallback(() => {
    api.get<{ capsules: CapsuleRow[] }>(`${V1}/capsules`).then(r => setRows(r.capsules)).catch(e => setError(errMsg(e)))
    api.get<HomeT>(`${V1}/home`).then(setHome).catch(() => {})
  }, [])
  useEffect(() => { load() }, [load])
  useEffect(() => { api.get<{ templates: Template[] }>(`${V1}/templates`).then(r => setTemplates(r.templates)).catch(() => {}) }, [])
  useOnChange(load)

  if (error) return <Empty title="Could not load your capsules" icon="alert">{error}</Empty>
  if (!rows) return <Loading />

  const ready = rows.filter(r => r.ready)
  const of = (s: Show) => s === 'all' ? rows : s === 'ready' ? ready : rows.filter(r => r.capsule.state === s && !r.ready)
  const sealed = of('sealed').sort((a, b) => a.capsule.opens_at.localeCompare(b.capsule.opens_at))
  const filling = of('filling')
  const opened = of('opened').sort((a, b) => (b.capsule.opened_at || '').localeCompare(a.capsule.opened_at || ''))
  const next = home?.next
  const count = (s: Show) => of(s).length

  if (rows.length === 0) {
    return (
      <div className="page narrow">
        <div className="hero-empty">
          <div className="hero-ico">⏳</div>
          <h1>Your first time capsule</h1>
          <p className="muted">Seal a letter, photos, a recording and a few guesses until a day you choose. Nobody can look inside until then — not even you.</p>
          <div className="tmpl-grid">
            {templates.slice(0, 8).map(t => (
              <button key={t.id} type="button" className="tmpl" onClick={() => navigate(`/new?template=${t.id}`)}>
                <span className="tmpl-emoji">{t.emoji}</span><span className="tmpl-name">{t.name}</span><span className="tmpl-about">{t.about}</span>
              </button>
            ))}
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="page">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Capsules</h1></div>
        <div className="scroll-x">
          <Segmented size="sm" value={show} onChange={v => setParams(v === 'all' ? {} : { show: v })} label="Show" options={[
            { value: 'all', label: `All ${rows.length}` }, { value: 'ready', label: `Ready ${count('ready')}`, disabled: !count('ready') },
            { value: 'filling', label: `Filling ${count('filling')}` }, { value: 'sealed', label: `Sealed ${count('sealed')}` }, { value: 'opened', label: `Opened ${count('opened')}` },
          ]} />
        </div>
      </div>

      {show === 'all' && ready.map(r => (
        <Link key={r.capsule.id} to={`/c/${r.capsule.id}`} className="ready-banner" style={capsuleStyle(r.capsule)}>
          <span className="ready-emoji">{r.capsule.emoji}</span>
          <span className="grow"><b>“{r.capsule.title}” is ready to open</b><span className="muted"> · sealed {r.capsule.sealed_at ? new Date(r.capsule.sealed_at).toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' }) : ''}</span></span>
          <span className="btn primary sm"><Icon name="gift" size={14} />Open it</span>
        </Link>
      ))}

      {show === 'all' && home?.yearly_due.map(y => (
        <div key={y.id} className="yearly-banner">
          <Icon name="repeat" size={16} />
          <span className="grow">Time for this year's capsule: <b>{y.title}</b></span>
          <button type="button" className="btn sm" onClick={() => navigate(`/new?ritual=${y.id}`)}>Write it</button>
        </div>
      ))}

      {show === 'all' && next && (
        <Link to={`/c/${next.capsule.id}`} className="next-card" style={capsuleStyle(next.capsule)}>
          <div className="next-left">
            <span className="next-label"><Icon name="lock" size={13} strokeWidth={2.2} />Opens next</span>
            <span className="next-title"><span aria-hidden>{next.capsule.emoji}</span> {next.capsule.title}</span>
            <span className="muted small">{opensWords(next.capsule)} · {next.capsule.items} {next.capsule.items === 1 ? 'thing' : 'things'} inside</span>
          </div>
          <Countdown to={next.capsule.opens_at} compact />
        </Link>
      )}

      {(show === 'all' || show === 'ready') && show === 'ready' && <Group title="Ready to open" rows={ready} />}
      {(show === 'all' || show === 'filling') && <Group title="Being filled" rows={filling} hint="Not sealed yet: add to them, then seal" />}
      {(show === 'all' || show === 'sealed') && <Group title="Sealed" rows={sealed} hint="Waiting for their day" />}
      {(show === 'all' || show === 'opened') && <Group title="Opened" rows={opened} />}
    </div>
  )
}

function Group({ title, rows, hint }: { title: string; rows: CapsuleRow[]; hint?: string }) {
  if (!rows.length) return null
  return (
    <section className="cap-group">
      <h2 className="group-h">{title}<span className="muted"> {rows.length}</span>{hint && <span className="group-hint">{hint}</span>}</h2>
      <div className="cap-grid">{rows.map(r => <CapsuleCard key={r.capsule.id} row={r} />)}</div>
    </section>
  )
}
