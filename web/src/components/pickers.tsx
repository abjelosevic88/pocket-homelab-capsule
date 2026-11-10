// Pick something from another Pocket app: people (with their birthdays),
// photos, notes, voice memos.
import { useEffect, useState } from 'react'
import { api, V1 } from '../lib/api'
import { Empty, Loading, Modal, Segmented, errMsg, useDebounce } from './ui'
import { Icon } from './icons'
import { useSibling } from './capsule'

export interface Person { name: string; email: string; birthday: string; anniversary?: string; from: 'family' | 'contacts' }

function Search({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder: string }) {
  return (
    <div className="add-search"><Icon name="search" size={15} /><input value={value} onChange={e => onChange(e.target.value)} placeholder={placeholder} autoFocus /></div>
  )
}

/** People from Pocket Family and Pocket Contacts. `need` = what the person must have. */
export function PersonPicker({ onPick, onClose, need, title = 'Pick someone' }: { onPick: (p: Person) => void; onClose: () => void; need?: 'email' | 'birthday' | 'anniversary'; title?: string }) {
  const fam = useSibling('family', 'family:read')
  const con = useSibling('contacts', 'contacts:read')
  const [src, setSrc] = useState<'family' | 'contacts'>(fam.ok || !con.ok ? 'family' : 'contacts')
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 250)
  const [list, setList] = useState<Person[] | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    setList(null); setErr('')
    if (src === 'family') {
      if (!fam.ok) { setErr(fam.why); return }
      api.get<{ people: { name: string; email: string; birthday: string }[] }>(`${V1}/apps/family`).then(r => setList(r.people.map(p => ({ ...p, from: 'family' as const })))).catch(e => setErr(errMsg(e)))
    } else {
      if (!con.ok) { setErr(con.why); return }
      api.get<{ contacts: { name: string; emails: string[]; birthday: string; anniversary: string }[] }>(`${V1}/apps/contacts${dq ? `?q=${encodeURIComponent(dq)}` : ''}`)
        .then(r => setList(r.contacts.map(c => ({ name: c.name, email: c.emails[0] || '', birthday: c.birthday, anniversary: c.anniversary, from: 'contacts' as const })))).catch(e => setErr(errMsg(e)))
    }
  }, [src, dq, fam.ok, con.ok]) // eslint-disable-line react-hooks/exhaustive-deps
  const shown = (list || []).filter(p => (!need || (need === 'email' ? p.email : need === 'birthday' ? p.birthday : p.anniversary)) && (!q || src === 'contacts' || p.name.toLowerCase().includes(q.toLowerCase())))
  return (
    <Modal title={title} onClose={onClose}>
      <div className="stack">
        <Segmented value={src} onChange={setSrc} options={[{ value: 'family', label: 'Pocket Family', disabled: !fam.ok, title: fam.ok ? undefined : fam.why }, { value: 'contacts', label: 'Pocket Contacts', disabled: !con.ok, title: con.ok ? undefined : con.why }]} />
        <Search value={q} onChange={setQ} placeholder="Search by name" />
        {err ? <p className="muted small">{err}</p> : !list ? <Loading /> : shown.length === 0 ? <Empty title="Nobody here" icon="users">{need === 'birthday' ? 'Only people with a birthday (with the year) show up.' : need === 'email' ? 'Only people with an email address show up.' : null}</Empty> : (
          <div className="pick-list">
            {shown.map((p, i) => (
              <button key={i} type="button" className="pick-row" onClick={() => onPick(p)}>
                <span className="grow"><b>{p.name}</b>{p.email && <span className="muted small"> · {p.email}</span>}</span>
                {need === 'anniversary' ? p.anniversary && <span className="muted small">💍 {p.anniversary}</span> : p.birthday && <span className="muted small">🎂 {p.birthday}</span>}
              </button>
            ))}
          </div>
        )}
      </div>
    </Modal>
  )
}

export function PhotoPicker({ capsuleId, onDone, onClose }: { capsuleId: number; onDone: (text: string) => void; onClose: () => void }) {
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 400)
  const [photos, setPhotos] = useState<{ id: string; name: string; date: string; thumb_url: string }[] | null>(null)
  const [picked, setPicked] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  useEffect(() => {
    setPhotos(null)
    api.get<{ photos: typeof photos }>(`${V1}/apps/photos${dq ? `?q=${encodeURIComponent(dq)}` : ''}`).then(r => setPhotos(r.photos || [])).catch(e => { setErr(errMsg(e)); setPhotos([]) })
  }, [dq])
  const toggle = (id: string) => setPicked(p => p.includes(id) ? p.filter(x => x !== id) : p.length < 30 ? [...p, id] : p)
  const add = async () => {
    setBusy(true); setErr('')
    try { const r = await api.post<{ text: string }>(`${V1}/apps/photos/copy`, { capsule_id: capsuleId, ids: picked }); onDone(r.text) } catch (e) { setErr(errMsg(e)); setBusy(false) }
  }
  return (
    <Modal title="Photos from Pocket Photos" onClose={onClose} size="lg" footer={<>
      <span className="muted small grow">{picked.length ? `${picked.length} picked — copies go in, so they are still there when it opens` : 'Pick up to 30'}</span>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" disabled={!picked.length || busy} onClick={add}>{busy ? 'Putting in…' : 'Put them in'}</button>
    </>}>
      <div className="stack">
        <Search value={q} onChange={setQ} placeholder="Search your photos (“beach”, “birthday”)" />
        {err && <p className="error-text">{err}</p>}
        {!photos ? <Loading /> : photos.length === 0 ? <Empty title="No photos" icon="image" /> : (
          <div className="photo-grid">
            {photos.map(p => (
              <button key={p.id} type="button" className={`photo-pick ${picked.includes(p.id) ? 'on' : ''}`} onClick={() => toggle(p.id)} title={`${p.name} · ${p.date}`}>
                <img src={p.thumb_url} alt="" loading="lazy" />
                {picked.includes(p.id) && <span className="photo-check"><Icon name="check" size={14} strokeWidth={2.6} /></span>}
              </button>
            ))}
          </div>
        )}
      </div>
    </Modal>
  )
}

export function NotePicker({ capsuleId, onDone, onClose }: { capsuleId: number; onDone: (text: string) => void; onClose: () => void }) {
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 300)
  const [notes, setNotes] = useState<{ id: string; title: string; folder: string; snippet: string }[] | null>(null)
  const [err, setErr] = useState('')
  useEffect(() => {
    setNotes(null)
    api.get<{ notes: typeof notes }>(`${V1}/apps/notes?q=${encodeURIComponent(dq)}`).then(r => setNotes(r.notes || [])).catch(e => { setErr(errMsg(e)); setNotes([]) })
  }, [dq])
  const pick = async (id: string) => {
    try { const r = await api.post<{ text: string }>(`${V1}/apps/notes/copy`, { capsule_id: capsuleId, note_id: id }); onDone(r.text) } catch (e) { setErr(errMsg(e)) }
  }
  return (
    <Modal title="A note from Pocket Notes" onClose={onClose}>
      <div className="stack">
        <Search value={q} onChange={setQ} placeholder="Search your notes" />
        {err && <p className="error-text">{err}</p>}
        {!notes ? <Loading /> : notes.length === 0 ? <Empty title="No notes found" icon="note" /> : (
          <div className="pick-list">{notes.map(n => <button key={n.id} type="button" className="pick-row col" onClick={() => pick(n.id)}><b>{n.title}</b><span className="muted small">{n.folder}{n.snippet ? ` · ${n.snippet}` : ''}</span></button>)}</div>
        )}
        <p className="muted small">A copy goes in as a letter: changing the note later does not change the capsule.</p>
      </div>
    </Modal>
  )
}

export function VoicePicker({ capsuleId, onDone, onClose }: { capsuleId: number; onDone: (text: string) => void; onClose: () => void }) {
  const [q, setQ] = useState('')
  const dq = useDebounce(q, 300)
  const [memos, setMemos] = useState<{ id: string; title: string; date: string; seconds: number; summary: string }[] | null>(null)
  const [busy, setBusy] = useState('')
  const [err, setErr] = useState('')
  useEffect(() => {
    setMemos(null)
    api.get<{ memos: typeof memos }>(`${V1}/apps/voice${dq ? `?q=${encodeURIComponent(dq)}` : ''}`).then(r => setMemos(r.memos || [])).catch(e => { setErr(errMsg(e)); setMemos([]) })
  }, [dq])
  const pick = async (id: string) => {
    setBusy(id); setErr('')
    try { const r = await api.post<{ text: string }>(`${V1}/apps/voice/copy`, { capsule_id: capsuleId, memo_id: id }); onDone(r.text) } catch (e) { setErr(errMsg(e)); setBusy('') }
  }
  return (
    <Modal title="A voice memo from Pocket Voice" onClose={onClose}>
      <div className="stack">
        <Search value={q} onChange={setQ} placeholder="Search what was said" />
        {err && <p className="error-text">{err}</p>}
        {!memos ? <Loading /> : memos.length === 0 ? <Empty title="No voice memos" icon="mic" /> : (
          <div className="pick-list">{memos.map(m => (
            <button key={m.id} type="button" className="pick-row col" disabled={!!busy} onClick={() => pick(m.id)}>
              <b>{m.title}</b><span className="muted small">{m.date} · {Math.floor(m.seconds / 60)}:{String(m.seconds % 60).padStart(2, '0')}{m.summary ? ` · ${m.summary}` : ''}</span>
              {busy === m.id && <span className="muted small">Copying the recording…</span>}
            </button>
          ))}</div>
        )}
      </div>
    </Modal>
  )
}
