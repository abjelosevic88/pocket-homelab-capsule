// How the guesses in opened capsules turned out — and whether you are
// right as often as you are sure — plus the goals.
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Item } from '../lib/types'
import { Bars } from '../components/Chart'
import { Empty, Loading, Segmented, errMsg } from '../components/ui'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'

interface Row { item: Item; capsule: { title: string; emoji: string; opened_at: string | null; sealed_at: string | null } }
interface Bucket { label: string; from: number; to: number; n: number; rate: number; avg_confidence: number }
interface Data { predictions: Row[]; goals: Row[]; buckets: Bucket[]; marked: number; unmarked: number; right_pct?: number; goals_pct?: number; goals_marked: number }

const OUT: Record<string, string> = { yes: 'Came true', partly: 'Partly', no: 'No', '': 'Not marked' }

export default function Predictions() {
  useCrumb(undefined)
  const [d, setD] = useState<Data | null>(null)
  const [err, setErr] = useState('')
  const [tab, setTab] = useState<'predictions' | 'goals'>('predictions')
  useEffect(() => { api.get<Data>(`${V1}/predictions`).then(setD).catch(e => setErr(errMsg(e))) }, [])
  if (err) return <Empty title="Could not load" icon="alert">{err}</Empty>
  if (!d) return <Loading />
  if (!d.predictions.length && !d.goals.length) return <Empty title="No guesses opened yet" icon="sparkles">Put a prediction or a goal in a capsule; when it opens you mark what came true, and they all add up here.</Empty>
  const rows = tab === 'predictions' ? d.predictions : d.goals
  const buckets = d.buckets.filter(b => b.n > 0)
  return (
    <div className="page">
      <div className="page-head"><div className="grow"><h1 className="page-title">Predictions</h1><p className="muted">From the capsules you have opened.</p></div></div>
      <div className="stats-row">
        <div className="stat"><span className="stat-l">Came true</span><span className="stat-v">{d.right_pct !== undefined ? `${Math.round(d.right_pct)}%` : '—'}</span><span className="stat-s small">of {d.marked} marked</span></div>
        <div className="stat"><span className="stat-l">Still to mark</span><span className="stat-v">{d.unmarked}</span><span className="stat-s small">guesses</span></div>
        <div className="stat"><span className="stat-l">Goals done</span><span className="stat-v">{d.goals_pct !== undefined ? `${Math.round(d.goals_pct)}%` : '—'}</span><span className="stat-s small">of {d.goals_marked} marked</span></div>
      </div>
      {buckets.length > 0 && (
        <section className="card pad-card stack">
          <h2 className="card-h">As sure as you were?</h2>
          <p className="muted small">How many of your guesses came true, by how sure you were when you sealed them. A partly counts half.</p>
          <Bars items={buckets.map(b => ({ label: b.label.replace(/ \(.*\)/, ''), value: Math.round(b.rate), hint: `${b.label}: ${Math.round(b.rate)}% came true, ${b.n} ${b.n === 1 ? 'guess' : 'guesses'}, you were ${Math.round(b.avg_confidence)}% sure on average` }))} format={v => `${v}%`} height={150} />
          <p className="muted small">{calibration(buckets)}</p>
        </section>
      )}
      <Segmented value={tab} onChange={setTab} options={[{ value: 'predictions', label: `Predictions ${d.predictions.length}` }, { value: 'goals', label: `Goals ${d.goals.length}` }]} />
      <section className="card">
        <div className="rows">
          {rows.map(r => (
            <Link key={r.item.id} to={`/c/${r.item.capsule_id}#item-${r.item.id}`} className="row">
              <span className={`outcome-mark o-${r.item.outcome || 'none'}`} title={OUT[r.item.outcome]}><Icon name={r.item.outcome === 'yes' ? 'check' : r.item.outcome === 'no' ? 'x' : r.item.outcome === 'partly' ? 'minus' : 'circle'} size={14} strokeWidth={2.4} /></span>
              <span className="grow"><span>{r.item.body}</span><span className="muted small">{r.capsule.emoji} {r.capsule.title}{tab === 'predictions' ? ` · ${r.item.confidence}% sure` : ''}</span></span>
              <span className="muted small nowrap">{OUT[r.item.outcome]}</span>
            </Link>
          ))}
        </div>
      </section>
    </div>
  )
}

function calibration(b: Bucket[]): string {
  const sure = b.find(x => x.from >= 81), unsure = b.find(x => x.to <= 40)
  if (sure && sure.n >= 2 && sure.rate < sure.avg_confidence - 20) return 'When you are very sure, you are right less often than you think.'
  if (unsure && unsure.n >= 2 && unsure.rate > unsure.avg_confidence + 25) return 'Your long shots come true more often than you expect.'
  return 'Mark more guesses as your capsules open, and this gets clearer.'
}
