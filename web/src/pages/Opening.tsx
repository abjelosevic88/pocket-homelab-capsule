// The opening: the sealed capsule, the seal broken, then what is inside one
// thing at a time (and the world of the day it was sealed), at your pace.
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { announceChange } from '../lib/data'
import type { CapsuleView } from '../lib/types'
import { dayOf, waited } from '../lib/capsule'
import { ItemCard, SnapshotView, capsuleStyle } from '../components/capsule'
import { Empty, Loading, errMsg, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'

export default function Opening() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const [v, setV] = useState<CapsuleView | null>(null)
  const [error, setError] = useState('')
  const [step, setStep] = useState(-1) // -1 = the seal, 0… = the things, n = the world
  const [breaking, setBreaking] = useState(false)
  useCrumb(v?.capsule.title, v ? `/c/${v.capsule.id}` : undefined)
  useEffect(() => { api.get<CapsuleView>(`${V1}/capsules/${id}`).then(x => { setV(x); if (x.capsule.state === 'opened') setStep(0) }).catch(e => setError(errMsg(e))) }, [id])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (!v || step < 0 || (e.target as HTMLElement).closest('input, textarea')) return
      if (e.key === 'ArrowRight' || e.key === ' ') { e.preventDefault(); setStep(s => Math.min(s + 1, total)) }
      if (e.key === 'ArrowLeft') { e.preventDefault(); setStep(s => Math.max(0, s - 1)) }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  if (error) return <Empty title="This capsule is not here" icon="hourglass">{error}</Empty>
  if (!v) return <Loading />
  const c = v.capsule
  const hasWorld = !!c.snapshot && Object.keys(c.snapshot).some(k => k !== 'taken_at' && k !== 'missing')
  const total = v.items.length + (hasWorld ? 1 : 0)

  if (c.state !== 'opened' && !v.ready) {
    return <Empty title="Not yet" icon="lock">“{c.title}” opens {v.opens_in}. <Link to={`/c/${c.id}`}>Back to it</Link></Empty>
  }

  const open = async () => {
    setBreaking(true)
    try {
      const x = await api.post<CapsuleView>(`${V1}/capsules/${c.id}/open`)
      announceChange()
      window.setTimeout(() => { setV(x); setStep(0); setBreaking(false) }, 900)
    } catch (e) { toast(errMsg(e), 'err'); setBreaking(false) }
  }

  if (step < 0) {
    return (
      <div className="opening" style={capsuleStyle(c)}>
        <div className={`envelope ${breaking ? 'breaking' : ''}`}>
          <div className="env-emoji" aria-hidden>{c.emoji}</div>
          <div className="env-seal" aria-hidden><Icon name="hourglass" size={26} strokeWidth={1.8} /></div>
        </div>
        <h1 className="open-title">{c.title}</h1>
        <p className="muted">Sealed {dayOf(c.sealed_at)} by {c.owner_name} — {waited(c)} ago.</p>
        <p className="muted small">{v.kinds.map(k => `${k.n} ${k.kind}${k.n === 1 ? '' : 's'}`).join(' · ')}</p>
        <button type="button" className="btn primary lg" disabled={breaking} onClick={open}><Icon name="unlock" size={17} />{breaking ? 'Opening…' : 'Break the seal'}</button>
      </div>
    )
  }

  const done = step >= total
  const it = v.items[step]
  return (
    <div className="reveal" style={capsuleStyle(c)}>
      <div className="reveal-top">
        <span className="muted small">{done ? 'All of it' : `${Math.min(step + 1, total)} of ${total}`}</span>
        <span className="spacer" />
        <Link to={`/c/${c.id}`} className="btn sm ghost">See everything at once</Link>
      </div>
      <div className="reveal-stage" key={step}>
        {done ? (
          <div className="reveal-end">
            <div className="hero-ico">{c.emoji}</div>
            <h2>That was everything</h2>
            <p className="muted">Now mark which guesses came true, answer the questions, and write back to the person who sealed it.</p>
            <button type="button" className="btn primary lg" onClick={() => navigate(`/c/${c.id}`)}>Carry on</button>
          </div>
        ) : it ? <ItemCard it={it} /> : (
          <section className="card pad-card stack">
            <h2 className="card-h"><Icon name="globe" size={16} /> The world on {dayOf(c.sealed_at)}</h2>
            {c.snapshot && <SnapshotView snap={c.snapshot} />}
          </section>
        )}
      </div>
      <div className="reveal-nav">
        <button type="button" className="btn" disabled={step === 0} onClick={() => setStep(s => Math.max(0, s - 1))}><Icon name="chevron-left" size={16} />Back</button>
        {!done && <button type="button" className="btn primary" onClick={() => setStep(s => s + 1)}>Next<Icon name="chevron-right" size={16} /></button>}
      </div>
    </div>
  )
}
