// One capsule, in whatever state it is: being filled (add, ask others,
// seal), sealed (the countdown, the slot), ready (open it) or opened (read,
// mark the guesses, then and now, write back, keep it elsewhere).
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, openAction } from '../lib/data'
import type { CapsuleView, HistoryEvent, Item, Kind, SectionInfo, Snapshot } from '../lib/types'
import { GOAL_OUTCOMES, KINDS, OUTCOMES, dayOf, kindWords, opensWords, waited } from '../lib/capsule'
import { AppButton, Countdown, ItemCard, Prose, SnapshotView, capsuleStyle, useAIReady, useSibling } from '../components/capsule'
import { ItemEditor, type EditStart } from '../components/ItemEditor'
import { NotePicker, PersonPicker, PhotoPicker, VoicePicker } from '../components/pickers'
import { Confirm, CopyField, Empty, Field, Loading, MenuButton, type MenuItem, Modal, Segmented, Switch, errMsg, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'
import { SectionPick } from './NewCapsule'

export default function CapsulePage() {
  const { id } = useParams()
  const [v, setV] = useState<CapsuleView | null>(null)
  const [error, setError] = useState('')
  const load = useCallback(() => {
    api.get<CapsuleView>(`${V1}/capsules/${id}`).then(x => { setV(x); setError('') }).catch(e => setError(errMsg(e)))
  }, [id])
  useEffect(() => { setV(null); load() }, [load])
  useCrumb(v?.capsule.title)
  if (error) return <Empty title="This capsule is not here" icon="hourglass">{error === 'not found' ? 'It was thrown away, or it is not yours to see.' : error}</Empty>
  if (!v) return <Loading />
  return <Capsule v={v} reload={() => { load(); announceChange() }} />
}

function Capsule({ v, reload }: { v: CapsuleView; reload: () => void }) {
  const c = v.capsule
  const { settings, user } = useApp()
  const { toast } = useToast()
  const navigate = useNavigate()
  const owner = v.role === 'owner'
  const [edit, setEdit] = useState<EditStart | null>(null)
  const [picker, setPicker] = useState<'' | 'photos' | 'notes' | 'voice'>('')
  const [modal, setModal] = useState<'' | 'seal' | 'break' | 'delete' | 'settings' | 'chat' | 'ask'>('')
  const [busy, setBusy] = useState('')
  const ai = useAIReady()
  const aiOn = ai.ready

  const run = async (key: string, fn: () => Promise<unknown>, ok?: string, url?: string) => {
    setBusy(key)
    try { const r = await fn() as { text?: string; url?: string } | undefined; toast(r?.text || ok || 'Done', 'ok', openAction(r?.url || url)); reload() } catch (e) { toast(errMsg(e), 'err') }
    setBusy('')
  }
  const saved = (msg: string) => { setEdit(null); setPicker(''); toast(msg); reload() }

  const menu = (): MenuItem[] => {
    const m: MenuItem[] = []
    if (owner && c.state !== 'opened') m.push({ label: 'Change the details', icon: 'edit', onSelect: () => setModal('settings') })
    if (owner && c.state !== 'opened') m.push({ label: c.calendar_url ? 'Put in Pocket Calendar again' : 'Put the day in Pocket Calendar', icon: 'calendar-plus', onSelect: () => run('cal', () => api.post(`${V1}/capsules/${c.id}/calendar`)) })
    m.push({ label: 'Tell a chat', icon: 'message', onSelect: () => setModal('chat') })
    if (owner && c.state === 'sealed') m.push('sep', { label: 'Break the seal early…', icon: 'unlock', danger: true, onSelect: () => setModal('break') })
    if (owner) m.push('sep', { label: c.state === 'sealed' ? 'Throw it away unopened…' : 'Delete…', icon: 'trash', danger: true, onSelect: () => setModal('delete') })
    else m.push('sep', { label: 'Leave this capsule', icon: 'log-out', onSelect: () => run('leave', () => api.del(`${V1}/capsules/${c.id}/members/${user?.id}`).then(() => navigate('/'))) })
    return m
  }

  return (
    <div className={`page cap-page st-${v.ready ? 'ready' : c.state}`} style={capsuleStyle(c)}>
      <header className="cap-hero">
        <div className="cap-hero-emoji" aria-hidden>{c.emoji}</div>
        <div className="grow">
          <h1 className="cap-title">{c.title}</h1>
          {c.about && <p className="cap-about">{c.about}</p>}
          <p className="cap-sub">
            {c.state === 'opened' ? <>Opened {dayOf(c.opened_at)}{c.sealed_at && <> · sealed {dayOf(c.sealed_at)} · waited {waited(c)}</>}</>
              : c.state === 'sealed' ? <>Sealed {dayOf(c.sealed_at)} · opens {opensWords(c)}</>
                : <>Opens {opensWords(c)}{c.open_rule !== 'date' && c.for_name && <> — {c.for_name}'s {c.for_age > 0 ? `${c.for_age}${['th', 'st', 'nd', 'rd'][(c.for_age % 100 - 20) % 10] || ['th', 'st', 'nd', 'rd'][c.for_age % 100] || 'th'} ` : ''}{c.open_rule}</>}</>}
            {!owner && <> · made by {c.owner_name}</>}
          </p>
          {c.early && <p className="early-note"><Icon name="unlock" size={13} />The seal was broken {c.early_reason ? `early: ${c.early_reason}` : 'before its day'}.</p>}
        </div>
        <MenuButton label="More" align="right" items={menu}><Icon name="more" size={18} /></MenuButton>
      </header>

      {c.state === 'filling' && <Filling v={v} onAdd={setEdit} onPick={setPicker} onSeal={() => setModal('seal')} run={run} busy={busy} reload={reload} onAsk={() => setModal('ask')} />}
      {c.state === 'sealed' && <Sealed v={v} onAdd={setEdit} onPick={setPicker} run={run} busy={busy} reload={reload} onAsk={() => setModal('ask')} />}
      {c.state === 'opened' && <Opened v={v} run={run} busy={busy} reload={reload} />}

      <History id={c.id} key={c.updated_at} />

      {edit && <ItemEditor c={c} start={edit} template={v.template} aiOn={aiOn} onClose={() => setEdit(null)} onSaved={saved} />}
      {picker === 'photos' && <PhotoPicker capsuleId={c.id} onDone={saved} onClose={() => setPicker('')} />}
      {picker === 'notes' && <NotePicker capsuleId={c.id} onDone={saved} onClose={() => setPicker('')} />}
      {picker === 'voice' && <VoicePicker capsuleId={c.id} onDone={saved} onClose={() => setPicker('')} />}
      {modal === 'seal' && <SealDialog v={v} onClose={() => setModal('')} onSealed={(cal) => { setModal(''); toast(cal ? 'Sealed. The opening day is in your calendar.' : 'Sealed. See you on the day.', 'ok', openAction(cal)); reload() }} />}
      {modal === 'break' && <BreakDialog v={v} onClose={() => setModal('')} onDone={() => { setModal(''); reload() }} />}
      {modal === 'settings' && <DetailsDialog v={v} onClose={() => setModal('')} onSaved={() => { setModal(''); toast('Saved'); reload() }} />}
      {modal === 'chat' && <ChatDialog v={v} onClose={() => setModal('')} />}
      {modal === 'ask' && <AskDialog v={v} onClose={() => setModal('')} onSent={() => { setModal(''); reload() }} />}
      {modal === 'delete' && <Confirm title={c.state === 'sealed' ? 'Throw it away unopened?' : 'Delete this capsule?'} message={<>“{c.title}” and everything in it ({c.items} {c.items === 1 ? 'thing' : 'things'}) will be gone for good{c.state === 'sealed' ? ', without anyone ever seeing inside' : ''}.</>}
        confirmLabel={c.state === 'sealed' ? 'Throw it away' : 'Delete'} onCancel={() => setModal('')} onConfirm={async () => {
          try { await api.del(`${V1}/capsules/${c.id}?confirm=1`); announceChange(); navigate('/') } catch (e) { toast(errMsg(e), 'err') }
        }} />}
    </div>
  )
}

type Run = (key: string, fn: () => Promise<unknown>, ok?: string, url?: string) => Promise<void>

// ---------- adding ----------

function AddBar({ v, onAdd, onPick, slot }: { v: CapsuleView; onAdd: (s: EditStart) => void; onPick: (p: 'photos' | 'notes' | 'voice') => void; slot?: boolean }) {
  const photos = useSibling('photos', 'photos:read')
  const notes = useSibling('notes', 'notes:read')
  const voice = useSibling('voice', 'voice:read')
  return (
    <section className={`card pad-card add-card ${slot ? 'slot' : ''}`}>
      <h2 className="card-h">{slot ? <><Icon name="inbox" size={16} /> Drop something in through the slot</> : 'Put something in'}</h2>
      {slot && <p className="muted small">It goes in sealed: nobody — you included — sees it until {new Date(v.capsule.opens_at).toLocaleDateString(undefined, { day: 'numeric', month: 'long', year: 'numeric' })}.</p>}
      <div className="kind-grid">
        {KINDS.map(k => <button key={k.id} type="button" className="kind-btn" onClick={() => onAdd({ kind: k.id })} title={k.hint}><Icon name={k.icon} size={18} /><span>{k.label}</span></button>)}
      </div>
      <div className="from-apps">
        <span className="muted small">From your apps:</span>
        <button type="button" className="btn sm" disabled={!photos.ok} title={photos.ok ? 'Copies go in' : photos.why} onClick={() => onPick('photos')}><Icon name="image" size={14} />Pocket Photos</button>
        <button type="button" className="btn sm" disabled={!notes.ok} title={notes.ok ? undefined : notes.why} onClick={() => onPick('notes')}><Icon name="note" size={14} />Pocket Notes</button>
        <button type="button" className="btn sm" disabled={!voice.ok} title={voice.ok ? undefined : voice.why} onClick={() => onPick('voice')}><Icon name="mic" size={14} />Pocket Voice</button>
      </div>
    </section>
  )
}

function Filling({ v, onAdd, onPick, onSeal, run, busy, reload, onAsk }: { v: CapsuleView; onAdd: (s: EditStart) => void; onPick: (p: 'photos' | 'notes' | 'voice') => void; onSeal: () => void; run: Run; busy: string; reload: () => void; onAsk: () => void }) {
  const c = v.capsule
  const owner = v.role === 'owner'
  const ai = useAIReady()
  const { toast } = useToast()
  const [ideas, setIdeas] = useState<{ prompts: string[]; questions: string[]; predictions: string[] } | null>(null)
  const [thinking, setThinking] = useState(false)
  const mine = (it: Item) => it.author_name
  const getIdeas = async () => {
    setThinking(true)
    try { setIdeas(await api.post(`${V1}/ai/ideas`, { capsule_id: c.id })) } catch (e) { toast(errMsg(e), 'err') }
    setThinking(false)
  }
  const t = v.template
  return (
    <>
      <div className="cap-status filling">
        <div className="grow">
          <b>Being filled</b>
          <span className="muted small">{v.items.length + v.hidden} {v.items.length + v.hidden === 1 ? 'thing' : 'things'} inside{v.hidden > 0 && ` · ${v.hidden} from others stay${v.hidden === 1 ? 's' : ''} a surprise until the opening`}</span>
        </div>
        <Countdown to={c.opens_at} compact />
        {owner && <button type="button" className="btn primary" onClick={onSeal} disabled={v.items.length + v.hidden === 0} title={v.items.length + v.hidden === 0 ? 'Put something in first' : undefined}><Icon name="lock" size={15} />Seal it</button>}
      </div>

      <AddBar v={v} onAdd={onAdd} onPick={onPick} />

      {(t.prompts.length > 0 || ai.on) && (
        <section className="card pad-card">
          <div className="row"><h2 className="card-h grow">Ideas</h2>{ai.on && <button type="button" className="btn sm" disabled={thinking || !ai.ready} title={ai.why || undefined} onClick={getIdeas}><Icon name="sparkles" size={14} />{thinking ? 'Thinking…' : 'More ideas from Pocket AI'}</button>}</div>
          <div className="ideas">
            {(ideas?.prompts || t.prompts).slice(0, 5).map(p => <button key={p} type="button" className="idea" onClick={() => onAdd({ kind: 'letter', title: p })}><Icon name="feather" size={14} />{p}</button>)}
            {(ideas?.questions || t.questions).slice(0, 3).map(p => <button key={p} type="button" className="idea" onClick={() => onAdd({ kind: 'question', body: p })}><Icon name="help" size={14} />{p}</button>)}
            {(ideas?.predictions || t.predictions).slice(0, 3).map(p => <button key={p} type="button" className="idea" onClick={() => onAdd({ kind: 'prediction', body: p })}><Icon name="sparkles" size={14} />{p}</button>)}
            {t.lists.slice(0, 2).map(l => <button key={l.title} type="button" className="idea" onClick={() => onAdd({ kind: 'list', title: l.title, entries: l.labels.map(x => ({ label: x, value: '' })) })}><Icon name="list" size={14} />{l.title}</button>)}
          </div>
        </section>
      )}

      <section className="items">
        {v.items.length === 0 && v.hidden === 0 && <Empty title="Nothing inside yet" icon="feather">Start with a letter: what is today like?</Empty>}
        {v.items.map(it => (
          <ItemCard key={it.id} it={it} actions={<ItemMenu it={it} v={v} onEdit={() => onAdd({ kind: it.kind, item: it })} reload={reload} />} />
        ))}
        {v.hidden > 0 && <div className="hidden-note"><Icon name="eye-off" size={16} />{v.hidden} more {v.hidden === 1 ? 'thing' : 'things'} from {v.from.filter(f => !v.items.some(i => mine(i) === f.name)).map(f => f.name).join(', ') || 'others'} — a surprise until it opens.</div>}
      </section>

      <People v={v} run={run} busy={busy} reload={reload} onAsk={onAsk} />
      {owner && <WorldToday v={v} reload={reload} />}
    </>
  )
}

function ItemMenu({ it, v, onEdit, reload }: { it: Item; v: CapsuleView; onEdit: () => void; reload: () => void }) {
  const { toast } = useToast()
  const { user } = useApp()
  const mine = it.user_id === user?.id
  const canEdit = v.capsule.state === 'filling' && (mine || (v.role === 'owner' && !v.capsule.surprise && it.guest))
  const canDel = v.capsule.state === 'filling' && (mine || (v.role === 'owner' && (it.guest || !v.capsule.surprise)))
  if (!canEdit && !canDel) return null
  return (
    <MenuButton label="Item" align="right" className="btn ghost icon-only sm" items={[
      ...(canEdit ? [{ label: 'Change', icon: 'edit', onSelect: onEdit }] : []),
      ...(canDel ? [{ label: 'Take it out', icon: 'trash', danger: true, onSelect: async () => { try { await api.del(`${V1}/items/${it.id}`); toast('Taken out'); reload() } catch (e) { toast(errMsg(e), 'err') } } }] : []),
    ]}><Icon name="more" size={16} /></MenuButton>
  )
}

// ---------- people ----------

function People({ v, run, busy, reload, onAsk }: { v: CapsuleView; run: Run; busy: string; reload: () => void; onAsk: () => void }) {
  const c = v.capsule
  const owner = v.role === 'owner'
  const { toast } = useToast()
  const [email, setEmail] = useState('')
  const [role, setRole] = useState<'contributor' | 'viewer'>('contributor')
  const [rname, setRname] = useState('')
  const [remail, setRemail] = useState('')
  const [pick, setPick] = useState(false)
  const mail = useSibling('email', 'mail:send')
  const people = useSibling('contacts', 'contacts:read').ok
  const fam = useSibling('family', 'family:read').ok
  const addMember = async () => { try { await api.post(`${V1}/capsules/${c.id}/members`, { email, role }); setEmail(''); toast('Added'); reload() } catch (e) { toast(errMsg(e), 'err') } }
  const addRec = async (name: string, mailTo: string) => { try { await api.post(`${V1}/capsules/${c.id}/recipients`, { name, email: mailTo }); setRname(''); setRemail(''); toast('Added'); reload() } catch (e) { toast(errMsg(e), 'err') } }
  const canGive = c.state === 'filling' || (c.state === 'sealed' && c.slot)
  return (
    <div className="grid two">
      <section className="card pad-card stack">
        <h2 className="card-h">Who fills it</h2>
        <div className="people">
          <div className="person"><span className="grow"><b>{c.owner_name}</b></span><span className="muted small">made it</span></div>
          {v.members.map(m => (
            <div key={m.user_id} className="person">
              <span className="grow"><b>{m.name || m.email}</b> <span className="muted small">{m.role === 'viewer' ? 'opens it with you' : 'adds to it'}</span></span>
              {v.from.find(f => f.name === m.name) && <span className="muted small">{v.from.find(f => f.name === m.name)!.n} in</span>}
              {owner && <button type="button" className="btn ghost icon-only sm" aria-label="Take out" onClick={() => run('rm', () => api.del(`${V1}/capsules/${c.id}/members/${m.user_id}`), 'Taken out')}><Icon name="x" size={14} /></button>}
            </div>
          ))}
        </div>
        {owner && c.state !== 'opened' && (
          <div className="row wrap">
            <input value={email} onChange={e => setEmail(e.target.value)} placeholder="Email of someone on this box" style={{ flex: '2 1 200px' }} />
            <Segmented size="sm" value={role} onChange={setRole} options={[{ value: 'contributor', label: 'Adds' }, { value: 'viewer', label: 'Only opens' }]} />
            <button type="button" className="btn" disabled={!email.includes('@')} onClick={addMember}>Add</button>
          </div>
        )}
        {owner && canGive && (
          <div className="give">
            <label className="check-row"><Switch checked={c.give_open} onChange={on => run('give', () => api.put(`${V1}/capsules/${c.id}/give`, { open: on }), on ? 'The link works now' : 'The link is off')} />
              <span><b>A link for people without an account</b><br /><span className="muted small">Grandparents, friends: they add a letter and a photo, and see nothing else.</span></span></label>
            {c.give_open && v.give_url && (
              <>
                <CopyField value={v.give_url} />
                <div className="row wrap">
                  <button type="button" className="btn sm" disabled={!mail.ok} title={mail.ok ? undefined : mail.why} onClick={onAsk}><Icon name="mail" size={14} />Email it with Pocket Email</button>
                  <button type="button" className="btn sm" onClick={() => run('newkey', () => api.put(`${V1}/capsules/${c.id}/give`, { new_key: true }), 'A new link; the old one stopped working')}><Icon name="refresh" size={14} />New link</button>
                </div>
              </>
            )}
          </div>
        )}
      </section>

      <section className="card pad-card stack">
        <h2 className="card-h">Who it is for</h2>
        {!owner ? <p className="muted small">{c.owner_name} chooses who gets it.</p> : (
          <>
            {v.recipients.length === 0 && <p className="muted small">Nobody yet — add people and each gets their own link: a countdown before, the whole capsule after.</p>}
            {v.recipients.map(r => (
              <div key={r.id} className="person">
                <span className="grow"><b>{r.name || r.email}</b> <span className="muted small">{r.email}</span><br />
                  <span className="muted small">{r.viewed_at ? `opened their link ${dayOf(r.viewed_at)}` : r.sent_at ? `emailed ${dayOf(r.sent_at)}` : 'not told yet'}</span></span>
                <MenuButton label="Recipient" align="right" className="btn ghost icon-only sm" items={[
                  { label: 'Copy their link', icon: 'copy', onSelect: () => { navigator.clipboard?.writeText(r.url); toast('Link copied') } },
                  { label: c.state === 'opened' ? 'Email them the capsule' : 'Email them what is waiting', icon: 'mail', disabled: !mail.ok, onSelect: () => run('send' + r.id, () => api.post(`${V1}/capsules/${c.id}/recipients/${r.id}/send`)) },
                  'sep',
                  { label: 'Take off the list', icon: 'trash', danger: true, onSelect: () => run('rmr', () => api.del(`${V1}/capsules/${c.id}/recipients/${r.id}`), 'Taken off') },
                ]}><Icon name="more" size={16} /></MenuButton>
              </div>
            ))}
            <div className="row wrap">
              <input value={rname} onChange={e => setRname(e.target.value)} placeholder="Name" style={{ flex: '1 1 120px' }} />
              <input value={remail} onChange={e => setRemail(e.target.value)} placeholder="Email" type="email" style={{ flex: '2 1 180px' }} />
              <button type="button" className="btn" disabled={!remail.includes('@')} onClick={() => addRec(rname, remail)}>Add</button>
              <button type="button" className="btn" disabled={!people && !fam} title={people || fam ? undefined : 'Connect Pocket Contacts or Family'} onClick={() => setPick(true)}><Icon name="users" size={14} />Pick</button>
            </div>
            {v.recipients.length > 0 && <p className="muted small">{c.deliver === 'on_date' ? 'They get it on the day itself, opened or not.' : 'They get it when the capsule is opened.'} {!mail.ok && 'Without Pocket Email, send them their link yourself.'}</p>}
          </>
        )}
        {busy && <span className="sr-only">Working</span>}
      </section>
      {pick && <PersonPicker need="email" title="Who is it for?" onClose={() => setPick(false)} onPick={p => { setPick(false); addRec(p.name, p.email) }} />}
    </div>
  )
}

// ---------- the world today ----------

function WorldToday({ v, reload }: { v: CapsuleView; reload: () => void }) {
  const c = v.capsule
  const { toast } = useToast()
  const [sections, setSections] = useState<SectionInfo[]>([])
  const [preview, setPreview] = useState<Snapshot | null>(null)
  const [loading, setLoading] = useState(false)
  useEffect(() => { api.get<{ sections: SectionInfo[] }>(`${V1}/sections`).then(r => setSections(r.sections)).catch(() => {}) }, [])
  const set = async (id: string, on: boolean) => {
    const next = on ? [...c.sections, id] : c.sections.filter(x => x !== id)
    try { await api.put(`${V1}/capsules/${c.id}`, { sections: next }); setPreview(null); reload() } catch (e) { toast(errMsg(e), 'err') }
  }
  const look = async () => {
    setLoading(true)
    try { setPreview((await api.get<{ snapshot: Snapshot }>(`${V1}/capsules/${c.id}/snapshot`)).snapshot) } catch (e) { toast(errMsg(e), 'err') }
    setLoading(false)
  }
  return (
    <section className="card pad-card stack">
      <div className="row"><h2 className="card-h grow">The world today</h2><button type="button" className="btn sm" disabled={loading || !c.sections.length} onClick={look}><Icon name="eye" size={14} />{loading ? 'Looking…' : 'What it would keep'}</button></div>
      <p className="muted small">When you seal it, these facts of the day are kept from your other apps — and read again on the day it opens, to compare.</p>
      <div className="section-picks">{sections.map(s => <SectionPick key={s.id} s={s} on={c.sections.includes(s.id)} onChange={on => set(s.id, on)} />)}</div>
      {preview && <><SnapshotView snap={preview} />{preview.missing && <p className="muted small">Left out: {Object.entries(preview.missing).map(([k, why]) => `${k} (${why})`).join('; ')}</p>}</>}
    </section>
  )
}

// ---------- sealed ----------

function Sealed({ v, onAdd, onPick, run, busy, reload, onAsk }: { v: CapsuleView; onAdd: (s: EditStart) => void; onPick: (p: 'photos' | 'notes' | 'voice') => void; run: Run; busy: string; reload: () => void; onAsk: () => void }) {
  const c = v.capsule
  const navigate = useNavigate()
  const total = v.kinds.reduce((n, k) => n + k.n, 0)
  return (
    <>
      <section className={`sealed-card ${v.ready ? 'ready' : ''}`}>
        <div className="seal-mark" aria-hidden><Icon name={v.ready ? 'gift' : 'lock'} size={30} strokeWidth={1.6} /></div>
        {v.ready ? (
          <>
            <h2>Its day has come</h2>
            <p className="muted">Sealed {dayOf(c.sealed_at)} — {waited(c)} ago. {total} {total === 1 ? 'thing is' : 'things are'} waiting inside.</p>
            <button type="button" className="btn primary lg" onClick={() => navigate(`/c/${c.id}/open`)}><Icon name="gift" size={17} />Open it</button>
          </>
        ) : (
          <>
            <h2>Sealed</h2>
            <Countdown to={c.opens_at} />
            <p className="muted">Opens {opensWords(c)}. Nobody can look inside before then{v.role === 'owner' ? ' — you neither' : ''}.</p>
          </>
        )}
        <div className="kinds">{v.kinds.map(k => <span key={k.kind} className="kind-chip">{kindWords(k.kind as Kind, k.n)}</span>)}</div>
        {v.from.length > 1 && <p className="muted small">From {v.from.map(f => f.name).join(', ')}</p>}
      </section>
      {v.can_add && !v.ready && <AddBar v={v} onAdd={onAdd} onPick={onPick} slot />}
      <People v={v} run={run} busy={busy} reload={reload} onAsk={onAsk} />
    </>
  )
}

// ---------- opened ----------

function Opened({ v, run, busy, reload }: { v: CapsuleView; run: Run; busy: string; reload: () => void }) {
  const c = v.capsule
  const { toast } = useToast()
  const ai = useAIReady()
  const navigate = useNavigate()
  const [now, setNow] = useState<Snapshot | null | undefined>(undefined)
  const [thenNow, setThenNow] = useState(c.then_now)
  const [writing, setWriting] = useState(false)
  const [reflection, setReflection] = useState(c.reflection)
  const [task, setTask] = useState<Item | null>(null)
  const canWrite = v.role !== 'viewer'
  useEffect(() => {
    if (c.snapshot && Object.keys(c.snapshot).some(k => k !== 'taken_at' && k !== 'missing')) api.get<{ now: Snapshot }>(`${V1}/capsules/${c.id}/now`).then(r => setNow(r.now)).catch(() => setNow(null))
  }, [c.id, c.snapshot])
  const mark = async (it: Item, patch: Record<string, string>) => { try { await api.put(`${V1}/items/${it.id}`, patch); reload() } catch (e) { toast(errMsg(e), 'err') } }
  const writeThenNow = async () => {
    setWriting(true)
    try { const r = await api.post<{ text: string }>(`${V1}/ai/then-now`, { capsule_id: c.id, now, save: true }); setThenNow(r.text); reload() } catch (e) { toast(errMsg(e), 'err') }
    setWriting(false)
  }
  const reply = async () => {
    try { const r = await api.post<CapsuleView>(`${V1}/capsules/${c.id}/reply`); announceChange(); navigate(`/c/${r.capsule.id}`) } catch (e) { toast(errMsg(e), 'err') }
  }
  const guesses = v.items.filter(i => i.kind === 'prediction')
  const right = guesses.filter(g => g.outcome).reduce((n, g) => n + (g.outcome === 'yes' ? 1 : g.outcome === 'partly' ? 0.5 : 0), 0)
  const marked = guesses.filter(g => g.outcome).length
  return (
    <>
      {(c.snapshot || thenNow) && (
        <section className="card pad-card stack then-now">
          <div className="row"><h2 className="card-h grow">Then and now</h2>
            {ai.on && canWrite && <button type="button" className="btn sm" disabled={writing || !ai.ready} title={ai.why || undefined} onClick={writeThenNow}><Icon name="sparkles" size={14} />{writing ? 'Writing…' : thenNow ? 'Write it again' : 'Write it with Pocket AI'}</button>}
          </div>
          {c.snapshot && <SnapshotView snap={c.snapshot} other={now && Object.keys(now).some(k => k !== 'taken_at' && k !== 'missing') ? now : undefined} labels={[`Sealed ${dayOf(c.sealed_at)}`, 'Today']} />}
          {thenNow && <Prose text={thenNow} />}
        </section>
      )}

      {guesses.length > 0 && <div className="guess-score"><Icon name="sparkles" size={16} />{marked === 0 ? `${guesses.length} ${guesses.length === 1 ? 'guess' : 'guesses'} to mark` : `${Math.round(right / marked * 100)}% of the guesses came true (${marked} of ${guesses.length} marked)`}<Link to="/predictions" className="spacer-left">All your predictions</Link></div>}

      <section className="items">
        {v.items.map(it => (
          <ItemCard key={it.id} it={it}>
            {(it.kind === 'prediction' || it.kind === 'goal') && canWrite && (
              <div className="outcome">
                <span className="muted small">{it.kind === 'goal' ? 'Did you do it?' : 'Did it come true?'}</span>
                <Segmented size="sm" value={it.outcome || null} onChange={o => mark(it, { outcome: o === it.outcome ? '' : o })} options={(it.kind === 'goal' ? GOAL_OUTCOMES : OUTCOMES).map(o => ({ value: o.id, label: o.label }))} />
                {it.kind === 'goal' && it.outcome !== 'yes' && <AppButton slug="tasks" scope="tasks:write" icon="check-square" className="btn sm" onClick={() => setTask(it)}>Make it a task</AppButton>}
              </div>
            )}
            {(it.kind === 'question' || it.answer) && <Answer it={it} canWrite={canWrite} onSave={a => mark(it, { answer: a })} />}
            {it.kind === 'letter' && canWrite && <div className="item-actions"><AppButton slug="blog" scope="blog:write" icon="feather" className="btn sm ghost" busy={busy === 'blog' + it.id} onClick={() => run('blog' + it.id, () => api.post(`${V1}/apps/blog`, { item_id: it.id }))}>Make it a blog post</AppButton></div>}
          </ItemCard>
        ))}
      </section>

      <section className="card pad-card stack">
        <h2 className="card-h">Write back to the past</h2>
        <p className="muted small">What would you tell the person who sealed this? It stays with the capsule.</p>
        {canWrite ? (
          <>
            <textarea value={reflection} onChange={e => setReflection(e.target.value)} rows={5} placeholder="Funny to read this…" />
            <div className="row end"><button type="button" className="btn" disabled={reflection === c.reflection} onClick={() => run('refl', () => api.put(`${V1}/capsules/${c.id}/reflection`, { reflection }), 'Kept')}>Keep it</button></div>
          </>
        ) : c.reflection ? <Prose text={c.reflection} /> : <p className="muted small">Nothing yet.</p>}
      </section>

      <section className="card pad-card">
        <h2 className="card-h">Keep it, share it, write the next one</h2>
        <div className="row wrap">
          <AppButton slug="notes" scope="notes:write" icon="note" busy={busy === 'notes'} onClick={() => run('notes', () => api.post(`${V1}/apps/notes`, { capsule_id: c.id }))}>Keep as a note</AppButton>
          <AppButton slug="files" scope="files:write" icon="download" busy={busy === 'files'} onClick={() => run('files', () => api.post(`${V1}/apps/files`, { capsule_id: c.id }))}>Copy to Pocket Files</AppButton>
          <button type="button" className="btn primary" onClick={reply}><Icon name="plus" size={15} />Write the next one</button>
        </div>
      </section>
      <People v={v} run={run} busy={busy} reload={reload} onAsk={() => {}} />
      {task && <TaskDialog it={task} onClose={() => setTask(null)} onDone={() => setTask(null)} />}
    </>
  )
}

function Answer({ it, canWrite, onSave }: { it: Item; canWrite: boolean; onSave: (a: string) => void }) {
  const [a, setA] = useState(it.answer)
  useEffect(() => setA(it.answer), [it.answer])
  if (!canWrite) return it.answer ? <blockquote className="answer">{it.answer}</blockquote> : null
  return (
    <div className="answer-edit">
      <input value={a} onChange={e => setA(e.target.value)} placeholder={it.kind === 'question' ? 'Your answer now' : 'A note from today'} onKeyDown={e => { if (e.key === 'Enter' && a !== it.answer) onSave(a) }} />
      <button type="button" className="btn sm" disabled={a === it.answer} onClick={() => onSave(a)}>Keep</button>
    </div>
  )
}

function TaskDialog({ it, onClose, onDone }: { it: Item; onClose: () => void; onDone: () => void }) {
  const { toast } = useToast()
  const [title, setTitle] = useState(it.body)
  const [due, setDue] = useState('')
  const [busy, setBusy] = useState(false)
  const go = async () => {
    setBusy(true)
    try { const r = await api.post<{ text: string; url: string }>(`${V1}/apps/tasks`, { item_id: it.id, title, due }); toast(r.text, 'ok', openAction(r.url)); onDone() } catch (e) { toast(errMsg(e), 'err'); setBusy(false) }
  }
  return (
    <Modal title="Still want to do it?" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={busy || !title.trim()} onClick={go}>Add to Pocket Tasks</button></>}>
      <div className="stack">
        <Field label="Task"><input value={title} onChange={e => setTitle(e.target.value)} autoFocus /></Field>
        <Field label="By (optional)"><input type="date" value={due} onChange={e => setDue(e.target.value)} /></Field>
      </div>
    </Modal>
  )
}

// ---------- dialogs ----------

function SealDialog({ v, onClose, onSealed }: { v: CapsuleView; onClose: () => void; onSealed: (calendar: string) => void }) {
  const c = v.capsule
  const { toast } = useToast()
  const { settings } = useApp()
  const [busy, setBusy] = useState(false)
  const cal = useSibling('calendar', 'calendar:write')
  const total = v.items.length + v.hidden
  const seal = async () => {
    setBusy(true)
    try { const r = await api.post<CapsuleView & { calendar_url?: string }>(`${V1}/capsules/${c.id}/seal`); announceChange(); onSealed(r.calendar_url || '') } catch (e) { toast(errMsg(e), 'err'); setBusy(false) }
  }
  return (
    <Modal title="Seal it?" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Not yet</button><button type="button" className="btn primary" disabled={busy} onClick={seal}><Icon name="lock" size={15} />{busy ? 'Sealing…' : 'Seal it'}</button></>}>
      <div className="stack seal-dialog">
        <div className="seal-big" aria-hidden>{c.emoji}</div>
        <p>{total} {total === 1 ? 'thing goes' : 'things go'} in, and nobody — you included — can see {total === 1 ? 'it' : 'them'} again until <b>{opensWords(c)}</b>{v.ready ? '' : ` (${v.opens_in})`}.</p>
        <ul className="seal-list">
          {c.sections.length > 0 && <li><Icon name="globe" size={14} />The world of today is kept: {c.sections.join(', ')}.</li>}
          {c.slot && <li><Icon name="inbox" size={14} />The slot stays open: things can still be dropped in, unseen.</li>}
          {settings?.calendar_on_seal && cal.ok && <li><Icon name="calendar" size={14} />The opening day goes in your Pocket Calendar.</li>}
          {v.recipients.length > 0 && <li><Icon name="mail" size={14} />{v.recipients.length} {v.recipients.length === 1 ? 'person gets' : 'people get'} it {c.deliver === 'on_date' ? 'on the day' : 'when it is opened'}.</li>}
          <li><Icon name="unlock" size={14} />If you ever must, you can break the seal early — it stays on record.</li>
        </ul>
      </div>
    </Modal>
  )
}

function BreakDialog({ v, onClose, onDone }: { v: CapsuleView; onClose: () => void; onDone: () => void }) {
  const { toast } = useToast()
  const [confirm, setConfirm] = useState('')
  const [reason, setReason] = useState('')
  const go = async () => { try { await api.post(`${V1}/capsules/${v.capsule.id}/break`, { confirm, reason }); announceChange(); onDone() } catch (e) { toast(errMsg(e), 'err') } }
  return (
    <Modal title="Break the seal early?" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Keep it sealed</button><button type="button" className="btn danger" disabled={confirm.trim().toLowerCase() !== 'open'} onClick={go}>Break the seal</button></>}>
      <div className="stack">
        <p>It is meant to open {v.opens_in}. Opening it now cannot be undone, and everyone in it will see that the seal was broken early.</p>
        <Field label="Why (optional, kept with it)"><input value={reason} onChange={e => setReason(e.target.value)} placeholder="The project ended sooner" /></Field>
        <Field label={<>Type <b>open</b> to go on</>}><input value={confirm} onChange={e => setConfirm(e.target.value)} autoFocus /></Field>
      </div>
    </Modal>
  )
}

function DetailsDialog({ v, onClose, onSaved }: { v: CapsuleView; onClose: () => void; onSaved: () => void }) {
  const c = v.capsule
  const { toast } = useToast()
  const sealed = c.state === 'sealed'
  const d = new Date(c.opens_at)
  const pad = (n: number) => String(n).padStart(2, '0')
  const [title, setTitle] = useState(c.title)
  const [about, setAbout] = useState(c.about)
  const [emoji, setEmoji] = useState(c.emoji)
  const [day, setDay] = useState(`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`)
  const [time, setTime] = useState(`${pad(d.getHours())}:${pad(d.getMinutes())}`)
  const [slot, setSlot] = useState(c.slot)
  const [surprise, setSurprise] = useState(c.surprise)
  const [deliver, setDeliver] = useState(c.deliver)
  const [remind, setRemind] = useState(c.remind_days)
  const [color, setColor] = useState(c.color)
  const save = async () => {
    const body: Record<string, unknown> = { title, about, slot, deliver, remind_days: remind, color }
    if (!sealed) { body.emoji = emoji; body.surprise = surprise }
    const changed = `${day}T${time}` !== `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
    if (changed) { body.opens_at = `${day}T${time}`; body.open_rule = 'date' }
    try { await api.put(`${V1}/capsules/${c.id}`, body); onSaved() } catch (e) { toast(errMsg(e), 'err') }
  }
  const colors = ['', '#7a4a9e', '#c2588a', '#d5a154', '#4a7a5e', '#3d6fb0', '#b0571d', '#5b6470']
  return (
    <Modal title="Details" onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={save}>Save</button></>}>
      <div className="stack">
        <div className="row"><Field label="Emoji" className="emoji-field"><input value={emoji} disabled={sealed} onChange={e => setEmoji(e.target.value)} maxLength={8} /></Field><Field label="Name" className="grow"><input value={title} onChange={e => setTitle(e.target.value)} /></Field></div>
        <Field label="On the outside"><input value={about} onChange={e => setAbout(e.target.value)} /></Field>
        <Field label="Colour" group><div className="swatches">{colors.map(col => <button key={col || 'none'} type="button" className={`swatch ${color === col ? 'on' : ''}`} style={{ background: col || 'var(--surface-2)' }} aria-label={col || 'No colour'} onClick={() => setColor(col)} />)}</div></Field>
        <div className="row wrap">
          <Field label="Opens on"><input type="date" value={day} onChange={e => setDay(e.target.value)} /></Field>
          <Field label="at"><input type="time" value={time} onChange={e => setTime(e.target.value)} /></Field>
        </div>
        {sealed && <p className="muted small">Sealed: it can wait longer, but not open sooner.</p>}
        <Field label="Remind me before it opens" group><Segmented size="sm" value={remind} onChange={setRemind} options={[{ value: 0, label: 'No' }, { value: 1, label: 'A day' }, { value: 7, label: 'A week' }, { value: 30, label: 'A month' }]} /></Field>
        <label className="check-row"><Switch checked={slot} onChange={setSlot} /><span><b>The slot is open</b><br /><span className="muted small">Things can be dropped in after sealing, unseen.</span></span></label>
        {!sealed && <label className="check-row"><Switch checked={surprise} onChange={setSurprise} /><span><b>Keep it a surprise</b><br /><span className="muted small">Everyone sees only what they put in until it opens.</span></span></label>}
        {v.recipients.length > 0 && <Field label="The people it is for get it" group><Segmented size="sm" value={deliver} onChange={setDeliver} options={[{ value: 'on_open', label: 'When opened' }, { value: 'on_date', label: 'On the day' }]} /></Field>}
      </div>
    </Modal>
  )
}

function ChatDialog({ v, onClose }: { v: CapsuleView; onClose: () => void }) {
  const { toast } = useToast()
  const chat = useSibling('chat', 'chat:write')
  const [chats, setChats] = useState<{ id: string; name: string; emoji: string }[] | null>(null)
  const [to, setTo] = useState('')
  const [text, setText] = useState('')
  useEffect(() => { if (chat.ok) api.get<{ conversations: { id: string; name: string; emoji: string }[] }>(`${V1}/apps/chats`).then(r => setChats(r.conversations)).catch(() => setChats([])) }, [chat.ok])
  const send = async () => { try { const r = await api.post<{ text: string; url: string }>(`${V1}/apps/chat`, { capsule_id: v.capsule.id, to, text }); toast(r.text, 'ok', openAction(r.url)); onClose() } catch (e) { toast(errMsg(e), 'err') } }
  return (
    <Modal title="Tell a chat" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!chat.ok} onClick={send}>Send</button></>}>
      {!chat.ok ? <p className="muted">{chat.why}</p> : !chats ? <Loading /> : (
        <div className="stack">
          <div className="pick-list">
            <button type="button" className={`pick-row ${to === '' ? 'on' : ''}`} onClick={() => setTo('')}>Your own notes chat</button>
            {chats.map(ch => <button key={ch.id} type="button" className={`pick-row ${to === ch.id ? 'on' : ''}`} onClick={() => setTo(ch.id)}>{ch.emoji} {ch.name}</button>)}
          </div>
          <Field label="A line (optional)"><input value={text} onChange={e => setText(e.target.value)} placeholder={v.capsule.state === 'opened' ? 'Our time capsule is open!' : 'The time capsule is sealed.'} /></Field>
        </div>
      )}
    </Modal>
  )
}

function AskDialog({ v, onClose, onSent }: { v: CapsuleView; onClose: () => void; onSent: () => void }) {
  const { toast } = useToast()
  const [list, setList] = useState<{ name: string; addr: string }[]>([])
  const [name, setName] = useState('')
  const [addr, setAddr] = useState('')
  const [text, setText] = useState('')
  const [pick, setPick] = useState(false)
  const [busy, setBusy] = useState(false)
  const send = async () => { setBusy(true); try { const r = await api.post<{ text: string }>(`${V1}/capsules/${v.capsule.id}/ask`, { to: list, text }); toast(r.text); onSent() } catch (e) { toast(errMsg(e), 'err'); setBusy(false) } }
  return (
    <Modal title="Ask people to add something" onClose={onClose} footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!list.length || busy} onClick={send}><Icon name="mail" size={15} />Send</button></>}>
      <div className="stack">
        <p className="muted small">They get an email from your account with the link. They add a letter or a photo and see nothing else.</p>
        {list.map((p, i) => <div key={i} className="row"><span className="grow"><b>{p.name || p.addr}</b> <span className="muted small">{p.addr}</span></span><button type="button" className="btn ghost icon-only sm" aria-label="Remove" onClick={() => setList(l => l.filter((_, j) => j !== i))}><Icon name="x" size={14} /></button></div>)}
        <div className="row wrap">
          <input value={name} onChange={e => setName(e.target.value)} placeholder="Name" style={{ flex: '1 1 120px' }} />
          <input value={addr} onChange={e => setAddr(e.target.value)} placeholder="Email" type="email" style={{ flex: '2 1 180px' }} />
          <button type="button" className="btn" disabled={!addr.includes('@')} onClick={() => { setList(l => [...l, { name, addr }]); setName(''); setAddr('') }}>Add</button>
          <button type="button" className="btn" onClick={() => setPick(true)}><Icon name="users" size={14} />Pick</button>
        </div>
        <Field label="Your message (optional)"><textarea rows={3} value={text} onChange={e => setText(e.target.value)} placeholder={`I am making a time capsule, “${v.capsule.title}”. Would you add a letter or a photo?`} /></Field>
      </div>
      {pick && <PersonPicker need="email" onClose={() => setPick(false)} onPick={p => { setPick(false); setList(l => [...l, { name: p.name, addr: p.email }]) }} />}
    </Modal>
  )
}

// ---------- history ----------

const WHAT: Record<string, string> = {
  made: 'made it', sealed: 'sealed it', added: 'added', opened: 'opened it', early: 'broke the seal early', reminded: 'sent a reminder', ready: 'its day came',
  delivered: 'sent it on', reflected: 'wrote back', member: 'added', recipient: 'made it for',
}

function History({ id }: { id: number }) {
  const [h, setH] = useState<HistoryEvent[] | null>(null)
  const [open, setOpen] = useState(false)
  useEffect(() => { if (open && !h) api.get<{ history: HistoryEvent[] }>(`${V1}/capsules/${id}/history`).then(r => setH(r.history)).catch(() => setH([])) }, [open, h, id])
  return (
    <details className="card history" onToggle={e => setOpen((e.target as HTMLDetailsElement).open)}>
      <summary className="pad-card"><Icon name="history" size={15} /> What happened to it</summary>
      {open && (!h ? <Loading /> : (
        <ol className="timeline">
          {h.map(e => <li key={e.id}><span className="tl-at">{new Date(e.at).toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' })}</span><span><b>{e.who}</b> {WHAT[e.what] || e.what}{e.detail && <> {e.detail}</>}</span></li>)}
        </ol>
      ))}
    </details>
  )
}
