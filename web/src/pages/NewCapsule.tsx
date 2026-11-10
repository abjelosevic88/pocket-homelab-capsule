// A new capsule: pick what kind, when it opens, who adds to it and who it
// is for. Everything can change until it is sealed.
import { useEffect, useMemo, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange } from '../lib/data'
import type { CapsuleView, Ritual, Section, SectionInfo, Template } from '../lib/types'
import { SECTION_ICON } from '../lib/capsule'
import { Field, Loading, Segmented, Switch, errMsg, useToast } from '../components/ui'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'
import { PersonPicker, type Person } from '../components/pickers'
import { useSibling } from '../components/capsule'

const pad = (n: number) => String(n).padStart(2, '0')
const isoDay = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`

export default function NewCapsule() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { settings } = useApp()
  const { toast } = useToast()
  useCrumb(undefined)
  const [templates, setTemplates] = useState<Template[] | null>(null)
  const [sections, setSections] = useState<SectionInfo[]>([])
  const [tmpl, setTmpl] = useState<Template | null>(null)
  const [ritual, setRitual] = useState<Ritual | null>(null)

  const [title, setTitle] = useState('')
  const [about, setAbout] = useState('')
  const [rule, setRule] = useState<'date' | 'birthday' | 'anniversary'>('date')
  const [day, setDay] = useState('')
  const [time, setTime] = useState('09:00')
  const [forName, setForName] = useState('')
  const [forDate, setForDate] = useState('')
  const [forAge, setForAge] = useState(18)
  const [together, setTogether] = useState(false)
  const [members, setMembers] = useState('')
  const [recipients, setRecipients] = useState<{ name: string; email: string }[]>([])
  const [picked, setPicked] = useState<Section[]>([])
  const [surprise, setSurprise] = useState(true)
  const [slot, setSlot] = useState(true)
  const [deliver, setDeliver] = useState<'on_open' | 'on_date'>('on_open')
  const [busy, setBusy] = useState(false)
  const [picker, setPicker] = useState<'' | 'for' | 'recipient'>('')
  const fam = useSibling('family', 'family:read')
  const con = useSibling('contacts', 'contacts:read')
  const people = fam.ok || con.ok

  useEffect(() => {
    api.get<{ templates: Template[] }>(`${V1}/templates`).then(r => setTemplates(r.templates)).catch(() => setTemplates([]))
    api.get<{ sections: SectionInfo[] }>(`${V1}/sections`).then(r => setSections(r.sections)).catch(() => {})
    const rid = Number(params.get('ritual'))
    if (rid) api.get<{ rituals: Ritual[] }>(`${V1}/rituals`).then(r => setRitual(r.rituals.find(x => x.id === rid) || null)).catch(() => {})
  }, [params])

  // pick the template from the address (or the yearly capsule)
  useEffect(() => {
    if (!templates?.length || tmpl) return
    const id = ritual?.template || params.get('template')
    const t = templates.find(x => x.id === id)
    if (t) choose(t, ritual)
  }, [templates, ritual]) // eslint-disable-line react-hooks/exhaustive-deps

  const choose = (t: Template, r?: Ritual | null) => {
    setTmpl(t)
    setTitle(r ? `${r.title} ${new Date().getFullYear()}` : t.id === 'blank' ? '' : t.name)
    setAbout('')
    const now = new Date()
    const years = r?.years || t.years || 1
    let d = new Date(now.getFullYear() + years, now.getMonth(), now.getDate())
    if (t.open_rule === 'newyear') d = new Date(now.getFullYear() + 1, 0, 1)
    setDay(isoDay(d))
    setTime(t.open_rule === 'newyear' ? '00:00' : `${pad(settings?.open_hour ?? 9)}:00`)
    setRule(t.open_rule === 'birthday' || t.open_rule === 'anniversary' ? t.open_rule : 'date')
    setForAge(t.years || 0)
    setTogether(t.together)
    setSurprise(t.together ? true : settings?.surprise ?? true)
    setSlot(settings?.slot ?? true)
    setPicked((settings?.sections || ['weather', 'reading', 'listening', 'places']) as Section[])
  }

  const opensPreview = useMemo(() => {
    if (rule === 'date') return day ? new Date(`${day}T${time || '09:00'}`) : null
    if (!/^\d{4}-\d{2}-\d{2}$/.test(forDate)) return null
    const [y, m, dd] = forDate.split('-').map(Number)
    const h = settings?.open_hour ?? 9
    if (forAge > 0) return new Date(y + forAge, m - 1, dd, h)
    const now = new Date()
    let t = new Date(now.getFullYear(), m - 1, dd, h)
    if (t <= now) t = new Date(now.getFullYear() + 1, m - 1, dd, h)
    return t
  }, [rule, day, time, forDate, forAge, settings?.open_hour])

  const create = async () => {
    if (!tmpl) return
    setBusy(true)
    try {
      const body: Record<string, unknown> = { template: tmpl.id, title: title.trim() || tmpl.name, about, sections: picked, surprise, slot, deliver, open_rule: rule,
        members: together ? members.split(/[\s,;]+/).filter(x => x.includes('@')) : [], recipients }
      if (rule === 'date') body.opens_at = `${day}T${time || '09:00'}`
      else { body.for_name = forName; body.for_date = forDate; body.for_age = forAge }
      const v = await api.post<CapsuleView>(`${V1}/capsules`, body)
      announceChange()
      navigate(`/c/${v.capsule.id}`, { replace: true })
    } catch (e) { toast(errMsg(e), 'err'); setBusy(false) }
  }

  if (!templates) return <Loading />
  if (!tmpl) {
    return (
      <div className="page narrow">
        <div className="page-head"><div className="grow"><h1 className="page-title">New capsule</h1><p className="muted">What kind of capsule? You can change everything until you seal it.</p></div></div>
        <div className="tmpl-grid">
          {templates.map(t => (
            <button key={t.id} type="button" className="tmpl" onClick={() => choose(t)}>
              <span className="tmpl-emoji">{t.emoji}</span><span className="tmpl-name">{t.name}</span><span className="tmpl-about">{t.about}</span>
            </button>
          ))}
        </div>
      </div>
    )
  }

  const pickPerson = (p: Person) => {
    if (picker === 'for') { setForName(p.name); setForDate(rule === 'anniversary' ? p.anniversary || '' : p.birthday) }
    else if (p.email && !recipients.some(r => r.email === p.email)) setRecipients(r => [...r, { name: p.name, email: p.email }])
    setPicker('')
  }
  const quick = (years: number) => { const n = new Date(); setDay(isoDay(new Date(n.getFullYear() + years, n.getMonth(), n.getDate()))) }

  return (
    <div className="page narrow">
      <div className="page-head">
        <div className="grow">
          <button type="button" className="linkish small" onClick={() => setTmpl(null)}><Icon name="arrow-left" size={13} /> Another kind</button>
          <h1 className="page-title"><span aria-hidden>{tmpl.emoji}</span> {tmpl.name}</h1>
          <p className="muted">{tmpl.about}</p>
        </div>
      </div>

      <section className="card pad-card stack">
        <Field label="Name"><input value={title} onChange={e => setTitle(e.target.value)} placeholder={tmpl.name} maxLength={120} autoFocus /></Field>
        <Field label="On the outside" help="Seen while it is sealed (the inside is not)."><input value={about} onChange={e => setAbout(e.target.value)} placeholder="Written on a rainy Sunday after the move" maxLength={600} /></Field>
      </section>

      <section className="card pad-card stack">
        <h2 className="card-h">When it opens</h2>
        <Segmented value={rule} onChange={setRule} options={[{ value: 'date', label: 'On a day' }, { value: 'birthday', label: 'On a birthday' }, { value: 'anniversary', label: 'On an anniversary' }]} />
        {rule === 'date' ? (
          <>
            <div className="row wrap">
              <Field label="Day"><input type="date" value={day} min={isoDay(new Date(Date.now() + 86400000))} onChange={e => setDay(e.target.value)} /></Field>
              <Field label="Time"><input type="time" value={time} onChange={e => setTime(e.target.value)} /></Field>
            </div>
            <div className="chips">{[1, 2, 5, 10, 18, 25].map(y => <button key={y} type="button" className="chip" onClick={() => quick(y)}>In {y} {y === 1 ? 'year' : 'years'}</button>)}</div>
          </>
        ) : (
          <>
            <div className="row wrap">
              <Field label={rule === 'birthday' ? 'Whose birthday' : 'Whose anniversary'}><input value={forName} onChange={e => setForName(e.target.value)} placeholder={rule === 'birthday' ? 'Mila' : 'Ana and me'} /></Field>
              <Field label={rule === 'birthday' ? 'Born on' : 'Married on'}><input type="date" value={forDate} onChange={e => setForDate(e.target.value)} /></Field>
              <Field label={rule === 'birthday' ? 'Opens on their' : 'Opens on the'} help={forAge === 0 ? 'the next one' : undefined}>
                <div className="len-input sm"><input type="number" min={0} max={120} value={forAge} onChange={e => setForAge(Math.max(0, Math.min(120, Number(e.target.value) || 0)))} /><span className="len-unit">{forAge === 0 ? 'next' : ordinal(forAge)}</span></div>
              </Field>
            </div>
            <div><button type="button" className="btn sm" disabled={!people} title={people ? undefined : 'Connect Pocket Family or Pocket Contacts (Settings → Connected apps)'} onClick={() => setPicker('for')}><Icon name="users" size={14} />From Family or Contacts</button></div>
          </>
        )}
        {opensPreview && <p className="opens-preview"><Icon name="hourglass" size={15} />Opens on <b>{opensPreview.toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' })}</b>{rule === 'date' ? ` at ${time}` : ''}</p>}
      </section>

      <section className="card pad-card stack">
        <h2 className="card-h">Who fills it</h2>
        <Segmented value={together ? 'together' : 'me'} onChange={v => setTogether(v === 'together')} options={[{ value: 'me', label: 'Just me' }, { value: 'together', label: 'Together with others' }]} />
        {together && (
          <>
            <Field label="People on this box who add to it" help="Their email addresses. People without an account can add a letter through a link, which you get once the capsule is made.">
              <input value={members} onChange={e => setMembers(e.target.value)} placeholder="ana@example.com, dino@example.com" />
            </Field>
            <label className="check-row"><Switch checked={surprise} onChange={setSurprise} /><span><b>Keep it a surprise</b><br /><span className="muted small">Everyone sees only what they put in until the capsule opens — you too.</span></span></label>
          </>
        )}
      </section>

      <section className="card pad-card stack">
        <h2 className="card-h">Who it is for</h2>
        <p className="muted small">Each gets their own link: it shows how long is left, and the whole capsule once it opens. Leave it empty if it is for you.</p>
        {recipients.map((r, i) => (
          <div key={i} className="row"><span className="grow"><b>{r.name || r.email}</b> <span className="muted small">{r.email}</span></span><button type="button" className="btn ghost icon-only sm" aria-label="Remove" onClick={() => setRecipients(x => x.filter((_, j) => j !== i))}><Icon name="x" size={14} /></button></div>
        ))}
        <RecipientAdd onAdd={r => setRecipients(x => [...x, r])} onPick={() => setPicker('recipient')} canPick={people} />
        {recipients.length > 0 && <Field label="They get it" group><Segmented value={deliver} onChange={setDeliver} options={[{ value: 'on_open', label: 'When it is opened' }, { value: 'on_date', label: 'On the day itself' }]} /></Field>}
      </section>

      <section className="card pad-card stack">
        <h2 className="card-h">The world today</h2>
        <p className="muted small">When you seal it, the capsule keeps a few facts of the day from your other Pocket apps, to compare when it opens.</p>
        <div className="section-picks">
          {sections.map(s => <SectionPick key={s.id} s={s} on={picked.includes(s.id)} onChange={v => setPicked(p => v ? [...p, s.id] : p.filter(x => x !== s.id))} />)}
        </div>
        <label className="check-row"><Switch checked={slot} onChange={setSlot} /><span><b>Keep the slot open after sealing</b><br /><span className="muted small">Things can still be dropped in — unseen — until it opens.</span></span></label>
      </section>

      <div className="row end">
        <button type="button" className="btn" onClick={() => navigate(-1)}>Cancel</button>
        <button type="button" className="btn primary" disabled={busy || (rule === 'date' ? !day : !forDate)} onClick={create}><Icon name="plus" size={15} />{busy ? 'Making…' : 'Make the capsule'}</button>
      </div>
      {picker && <PersonPicker need={picker === 'for' ? (rule === 'anniversary' ? 'anniversary' : 'birthday') : 'email'} title={picker === 'for' ? 'Whose day?' : 'Who is it for?'} onPick={pickPerson} onClose={() => setPicker('')} />}
    </div>
  )
}

function ordinal(n: number) { const s = ['th', 'st', 'nd', 'rd'], v = n % 100; return n + (s[(v - 20) % 10] || s[v] || s[0]) }

export function SectionPick({ s, on, onChange }: { s: SectionInfo; on: boolean; onChange: (v: boolean) => void }) {
  const sib = useSibling(s.slug, s.scope)
  return (
    <label className={`section-pick ${sib.ok ? '' : 'off'}`} title={sib.ok ? undefined : sib.why}>
      <Switch checked={on && sib.ok} onChange={onChange} disabled={!sib.ok} size="sm" />
      <Icon name={SECTION_ICON[s.id]} size={15} />
      <span className="grow">{s.name}<span className="muted small"> · {s.app}</span></span>
    </label>
  )
}

function RecipientAdd({ onAdd, onPick, canPick }: { onAdd: (r: { name: string; email: string }) => void; onPick: () => void; canPick: boolean }) {
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const add = () => { if (email.includes('@')) { onAdd({ name: name.trim(), email: email.trim() }); setName(''); setEmail('') } }
  return (
    <div className="row wrap">
      <input value={name} onChange={e => setName(e.target.value)} placeholder="Name" style={{ flex: '1 1 140px' }} />
      <input value={email} onChange={e => setEmail(e.target.value)} placeholder="Email" type="email" style={{ flex: '2 1 200px' }} onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); add() } }} />
      <button type="button" className="btn" disabled={!email.includes('@')} onClick={add}>Add</button>
      <button type="button" className="btn" disabled={!canPick} title={canPick ? undefined : 'Connect Pocket Family or Pocket Contacts (Settings → Connected apps)'} onClick={onPick}><Icon name="users" size={14} />Pick</button>
    </div>
  )
}
