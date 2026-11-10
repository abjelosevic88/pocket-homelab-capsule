// Yearly capsules: on a day of the year, a nudge to write the next one.
import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import type { Ritual, Template } from '../lib/types'
import { Empty, Field, Loading, Modal, Switch, errMsg, useToast } from '../components/ui'
import { Select } from '../components/Select'
import { Icon } from '../components/icons'
import { useCrumb } from '../components/Layout'

const MONTHS = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December']

export default function Yearly() {
  useCrumb(undefined)
  const navigate = useNavigate()
  const { toast } = useToast()
  const [list, setList] = useState<Ritual[] | null>(null)
  const [templates, setTemplates] = useState<Template[]>([])
  const [edit, setEdit] = useState<Partial<Ritual> | null>(null)
  const load = useCallback(() => { api.get<{ rituals: Ritual[] }>(`${V1}/rituals`).then(r => setList(r.rituals)).catch(e => toast(errMsg(e), 'err')) }, [toast])
  useEffect(() => { load(); api.get<{ templates: Template[] }>(`${V1}/templates`).then(r => setTemplates(r.templates)).catch(() => {}) }, [load])
  const save = async (r: Partial<Ritual>) => {
    try { await (r.id ? api.put(`${V1}/rituals/${r.id}`, r) : api.post(`${V1}/rituals`, r)); setEdit(null); load() } catch (e) { toast(errMsg(e), 'err') }
  }
  if (!list) return <Loading />
  const tmpl = (id: string) => templates.find(t => t.id === id)
  return (
    <div className="page narrow">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Yearly capsules</h1><p className="muted">On the day, you get a nudge (and a banner here) to write this year's capsule.</p></div>
        <button type="button" className="btn primary" onClick={() => setEdit({ title: 'New Year\'s capsule', template: 'newyear', month: 12, day: 31, years: 1, active: true })}><Icon name="plus" size={15} />Add</button>
      </div>
      {list.length === 0 ? <Empty title="None yet" icon="repeat">A New Year's capsule on 31 December, a birthday letter on your birthday…</Empty> : (
        <section className="card">
          <div className="rows">
            {list.map(r => (
              <div key={r.id} className="row">
                <span className="sb-emoji" aria-hidden>{tmpl(r.template)?.emoji || '⏳'}</span>
                <span className="grow"><b>{r.title}</b><span className="muted small">Every {r.day} {MONTHS[r.month - 1]} · opens {r.years === 1 ? 'a year' : `${r.years} years`} later{r.last_year ? ` · last nudge ${r.last_year}` : ''}</span></span>
                <button type="button" className="btn sm" onClick={() => navigate(`/new?ritual=${r.id}`)}>Write it now</button>
                <Switch checked={r.active} onChange={on => save({ ...r, active: on })} label="On" />
                <button type="button" className="btn ghost icon-only sm" aria-label="Change" onClick={() => setEdit(r)}><Icon name="edit" size={15} /></button>
              </div>
            ))}
          </div>
        </section>
      )}
      {edit && (
        <Modal title={edit.id ? 'Change the yearly capsule' : 'A yearly capsule'} onClose={() => setEdit(null)} size="sm" footer={<>
          {edit.id && <button type="button" className="btn danger ghost" onClick={async () => { await api.del(`${V1}/rituals/${edit.id}`).catch(() => {}); setEdit(null); load() }}>Delete</button>}
          <span className="grow" />
          <button type="button" className="btn" onClick={() => setEdit(null)}>Cancel</button>
          <button type="button" className="btn primary" onClick={() => save(edit)}>Save</button>
        </>}>
          <div className="stack">
            <Field label="Name"><input value={edit.title || ''} onChange={e => setEdit({ ...edit, title: e.target.value })} /></Field>
            <Field label="Kind" group><Select value={edit.template || 'self'} onChange={v => setEdit({ ...edit, template: v })} options={templates.map(t => ({ value: t.id, label: `${t.emoji} ${t.name}` }))} /></Field>
            <div className="row">
              <Field label="Day" group><Select value={String(edit.day || 1)} onChange={v => setEdit({ ...edit, day: Number(v) })} options={Array.from({ length: 31 }, (_, i) => ({ value: String(i + 1), label: String(i + 1) }))} /></Field>
              <Field label="Month" group className="grow"><Select value={String(edit.month || 1)} onChange={v => setEdit({ ...edit, month: Number(v) })} options={MONTHS.map((m, i) => ({ value: String(i + 1), label: m }))} /></Field>
            </div>
            <Field label="It opens" group><Select value={String(edit.years || 1)} onChange={v => setEdit({ ...edit, years: Number(v) })} options={[1, 2, 3, 5, 10].map(y => ({ value: String(y), label: y === 1 ? 'a year later' : `${y} years later` }))} /></Field>
          </div>
        </Modal>
      )}
    </div>
  )
}
