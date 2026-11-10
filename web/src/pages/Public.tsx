// Pages for people without an account: a recipient's own link (a
// countdown, then the capsule) and the "add something" link.
import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, V1, ApiError } from '../lib/api'
import type { Item, Snapshot } from '../lib/types'
import { dayOf } from '../lib/capsule'
import { shrink } from '../lib/upload'
import { Countdown, ItemCard, Prose, SnapshotView } from '../components/capsule'
import { Field, Loading, errMsg } from '../components/ui'
import { Icon } from '../components/icons'

interface Outside { title: string; about: string; emoji: string; color: string; owner_name: string; state: string; opens_at: string; opens_in: string; sealed_at: string | null; opened_at: string | null; for_name: string; items: number }

function Shell({ color, children }: { color?: string; children: React.ReactNode }) {
  return (
    <div className="public" style={color ? ({ '--cap': color } as React.CSSProperties) : undefined}>
      <div className="public-inner">{children}</div>
      <footer className="public-foot"><img src="/favicon.svg" alt="" width={16} height={16} /> Pocket Capsule</footer>
    </div>
  )
}

export function RecipientPage() {
  const { key } = useParams()
  const [d, setD] = useState<{ capsule: Outside; name: string; open: boolean; items?: Item[]; reflection?: string; then_now?: string; snapshot?: Snapshot } | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => { api.get<typeof d>(`${V1}/public/o/${key}`).then(setD).catch(e => setErr(errMsg(e))) }, [key])
  if (err) return <Shell><div className="public-card"><h1>This link does not work</h1><p className="muted">{err}</p></div></Shell>
  if (!d) return <Shell><Loading /></Shell>
  const c = d.capsule
  const up = (id: number, thumb = false) => `${V1}/public/o/${key}/uploads/${id}${thumb ? '?thumb=1' : ''}`
  if (!d.open) {
    return (
      <Shell color={c.color}>
        <div className="public-card center">
          <div className="hero-ico">{c.emoji}</div>
          <p className="muted">{d.name ? `${d.name}, ` : ''}{c.owner_name} is keeping a time capsule for you</p>
          <h1>{c.title}</h1>
          {c.about && <p>{c.about}</p>}
          {c.state === 'sealed' ? <><Countdown to={c.opens_at} /><p className="muted">Sealed {dayOf(c.sealed_at)} · opens {new Date(c.opens_at).toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}</p></>
            : <p className="muted">It is still being filled. It opens {new Date(c.opens_at).toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' })}.</p>}
          <p className="muted small"><Icon name="lock" size={13} /> Nobody can see inside until then. Keep this link: it shows the capsule once it opens.</p>
        </div>
      </Shell>
    )
  }
  return (
    <Shell color={c.color}>
      <header className="public-head">
        <div className="hero-ico">{c.emoji}</div>
        <h1>{c.title}</h1>
        <p className="muted">From {c.owner_name}{c.sealed_at && <> · sealed {dayOf(c.sealed_at)}</>}{c.opened_at && <> · opened {dayOf(c.opened_at)}</>}</p>
        {c.about && <p>{c.about}</p>}
      </header>
      <div className="items">{(d.items || []).map(it => <ItemCard key={it.id} it={it} upload={up}>{it.outcome && <p className="muted small">{it.kind === 'goal' ? 'Done' : 'Came true'}: {it.outcome}</p>}{it.answer && <blockquote className="answer">{it.answer}</blockquote>}</ItemCard>)}</div>
      {d.snapshot && <section className="card pad-card stack"><h2 className="card-h">The world when it was sealed</h2><SnapshotView snap={d.snapshot} /></section>}
      {d.then_now && <section className="card pad-card"><h2 className="card-h">Then and now</h2><Prose text={d.then_now} /></section>}
      {d.reflection && <section className="card pad-card"><h2 className="card-h">Written back after opening</h2><Prose text={d.reflection} /></section>}
    </Shell>
  )
}

export function GivePage() {
  const { key } = useParams()
  const [d, setD] = useState<{ capsule: Outside; accepting: boolean; prompts: string[] } | null>(null)
  const [err, setErr] = useState('')
  const [name, setName] = useState(() => localStorage.getItem('pcsl-guest-name') || '')
  const [title, setTitle] = useState('')
  const [letter, setLetter] = useState('')
  const [photo, setPhoto] = useState<File | null>(null)
  const [busy, setBusy] = useState(false)
  const [sent, setSent] = useState(false)
  const [formErr, setFormErr] = useState('')
  useEffect(() => { api.get<typeof d>(`${V1}/public/give/${key}`).then(setD).catch(e => setErr(errMsg(e))) }, [key])
  if (err) return <Shell><div className="public-card"><h1>This link does not work</h1><p className="muted">{err}</p></div></Shell>
  if (!d) return <Shell><Loading /></Shell>
  const c = d.capsule
  const send = async () => {
    setBusy(true); setFormErr('')
    try {
      const fd = new FormData()
      fd.append('name', name.trim()); fd.append('title', title); fd.append('letter', letter)
      if (photo) {
        const s = await shrink(photo)
        if (s) { fd.append('photo', s.big, s.name); fd.append('thumb', s.thumb, 'thumb.jpg'); fd.append('width', String(s.w)); fd.append('height', String(s.h)) } else fd.append('photo', photo)
      }
      const res = await fetch(`${V1}/public/give/${key}`, { method: 'POST', body: fd })
      const j = await res.json().catch(() => ({}))
      if (!res.ok) throw new ApiError(res.status, j.error || res.statusText, j)
      localStorage.setItem('pcsl-guest-name', name.trim())
      setSent(true); setLetter(''); setTitle(''); setPhoto(null)
    } catch (e) { setFormErr(errMsg(e)) }
    setBusy(false)
  }
  return (
    <Shell color={c.color}>
      <div className="public-card">
        <div className="center">
          <div className="hero-ico">{c.emoji}</div>
          <p className="muted">{c.owner_name} asks you to add to a time capsule</p>
          <h1>{c.title}</h1>
          {c.about && <p>{c.about}</p>}
          <p className="muted small"><Icon name="lock" size={13} /> It opens {new Date(c.opens_at).toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' })}. Until then nobody sees what you put in — not even {c.owner_name}.</p>
        </div>
        {!d.accepting ? <p className="muted center">This capsule does not take anything new.</p> : sent ? (
          <div className="center stack">
            <h2>Thank you — it is in</h2>
            <p className="muted">It stays sealed until the day it opens.</p>
            <button type="button" className="btn" onClick={() => setSent(false)}>Add something else</button>
          </div>
        ) : (
          <div className="stack">
            <Field label="Your name"><input value={name} onChange={e => setName(e.target.value)} placeholder="Nona Vesna" maxLength={120} /></Field>
            <Field label="Title (optional)"><input value={title} onChange={e => setTitle(e.target.value)} maxLength={200} /></Field>
            <Field label="Your letter"><textarea value={letter} onChange={e => setLetter(e.target.value)} rows={8} placeholder={d.prompts[0] || 'Write what you would like them to read one day…'} /></Field>
            {d.prompts.length > 1 && <div className="chips">{d.prompts.slice(0, 4).map(p => <button key={p} type="button" className="chip" onClick={() => setTitle(p)}>{p}</button>)}</div>}
            <Field label="A photo (optional)"><input type="file" accept="image/*" onChange={e => setPhoto(e.target.files?.[0] || null)} /></Field>
            {formErr && <p className="error-text">{formErr}</p>}
            <button type="button" className="btn primary lg" disabled={busy || !name.trim() || (!letter.trim() && !photo)} onClick={send}><Icon name="send" size={16} />{busy ? 'Putting it in…' : 'Put it in the capsule'}</button>
          </div>
        )}
      </div>
    </Shell>
  )
}
