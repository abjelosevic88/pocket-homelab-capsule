// Put something in (or change what you put in): one dialog for every kind.
import { useEffect, useRef, useState } from 'react'
import { api, V1 } from '../lib/api'
import { uploadFile } from '../lib/upload'
import type { Capsule, Entry, Item, Kind, Template } from '../lib/types'
import { kindOf } from '../lib/capsule'
import { Field, Modal, errMsg, useToast } from './ui'
import { Icon } from './icons'

export interface EditStart { kind: Kind; item?: Item; title?: string; body?: string; entries?: Entry[] }

export function ItemEditor({ c, start, template, aiOn, onClose, onSaved }: { c: Capsule; start: EditStart; template?: Template; aiOn: boolean; onClose: () => void; onSaved: (msg: string) => void }) {
  const { toast } = useToast()
  const it = start.item
  const kind = start.kind
  const k = kindOf(kind)
  const [title, setTitle] = useState(it?.title ?? start.title ?? '')
  const [body, setBody] = useState(it?.body ?? start.body ?? '')
  const [conf, setConf] = useState(it?.confidence ?? 60)
  const [url, setUrl] = useState(it?.url ?? '')
  const [entries, setEntries] = useState<Entry[]>(it?.entries?.length ? it.entries : start.entries || [{ label: '', value: '' }, { label: '', value: '' }, { label: '', value: '' }])
  const [file, setFile] = useState<File | null>(null)
  const [uploadId, setUploadId] = useState<number | null>(it?.upload_id ?? null)
  const [busy, setBusy] = useState(false)
  const [ai, setAi] = useState('')
  const sealed = c.state === 'sealed'
  const needsFile = ['photo', 'audio', 'video', 'file'].includes(kind)

  const save = async () => {
    setBusy(true)
    try {
      let up = uploadId
      if (file) up = (await uploadFile(file, c.id)).upload.id
      const payload: Record<string, unknown> = { kind, title, body, confidence: conf, url, entries: entries.filter(e => e.label.trim() || e.value.trim()) }
      if (needsFile) payload.upload_id = up
      if (it) await api.put(`${V1}/items/${it.id}`, payload)
      else await api.post(`${V1}/capsules/${c.id}/items`, payload)
      onSaved(it ? 'Saved' : sealed ? 'Dropped in through the slot — it stays sealed until the day' : `${k.label} put in`)
    } catch (e) { toast(errMsg(e), 'err'); setBusy(false) }
  }

  const help = async (mode: 'draft' | 'tidy' | 'more') => {
    setAi(mode)
    try {
      const r = await api.post<{ text: string }>(`${V1}/ai/write`, { capsule_id: c.id, mode, prompt: title, text: body })
      setBody(b => mode === 'more' ? (b.trim() ? b.trim() + '\n\n' : '') + r.text : r.text)
    } catch (e) { toast(errMsg(e), 'err') }
    setAi('')
  }

  const ready = needsFile ? !!(file || uploadId) : kind === 'list' ? entries.some(e => e.value.trim()) : kind === 'link' ? /^https?:\/\//.test(url) : body.trim() !== ''
  return (
    <Modal title={<><Icon name={k.icon} size={17} /> {it ? `Change the ${k.label.toLowerCase()}` : k.label}</>} onClose={onClose} size={kind === 'letter' ? 'lg' : ''} footer={<>
      {sealed && <span className="muted small grow"><Icon name="lock" size={13} /> Sealed: nobody sees it until {new Date(c.opens_at).toLocaleDateString()}, you neither.</span>}
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" disabled={!ready || busy} onClick={save}>{busy ? 'Saving…' : it ? 'Save' : sealed ? 'Drop it in' : 'Put it in'}</button>
    </>}>
      <div className="stack" onKeyDown={e => { if ((e.metaKey || e.ctrlKey) && e.key === 'Enter' && ready && !busy) { e.preventDefault(); save() } }}>
        {kind === 'letter' && (
          <>
            <Field label="Title (optional)"><input value={title} onChange={e => setTitle(e.target.value)} placeholder="Dear future me" maxLength={200} /></Field>
            <Field label="The letter"><textarea className="letter-area" value={body} onChange={e => setBody(e.target.value)} rows={12} placeholder="Write what today is like, what you hope for, what you want to remember…" autoFocus /></Field>
            {aiOn && (
              <div className="ai-row">
                <Icon name="sparkles" size={14} /><span className="muted small">Help from Pocket AI:</span>
                <button type="button" className="btn sm" disabled={!!ai} onClick={() => help('draft')}>{ai === 'draft' ? 'Writing…' : body.trim() ? 'Draft from my notes' : 'A first draft'}</button>
                <button type="button" className="btn sm" disabled={!!ai || !body.trim()} onClick={() => help('more')}>{ai === 'more' ? 'Writing…' : 'Next paragraph'}</button>
                <button type="button" className="btn sm" disabled={!!ai || !body.trim()} onClick={() => help('tidy')}>{ai === 'tidy' ? 'Tidying…' : 'Tidy it'}</button>
              </div>
            )}
            {!it && template?.prompts?.length ? <div className="chips">{template.prompts.map(p => <button key={p} type="button" className="chip" onClick={() => setTitle(p)}>{p}</button>)}</div> : null}
          </>
        )}
        {kind === 'prediction' && (
          <>
            <Field label="Your guess" help="Something that will clearly be true or not on the day it opens."><input value={body} onChange={e => setBody(e.target.value)} placeholder="I will have changed jobs" autoFocus maxLength={500} /></Field>
            <Field label={`How sure are you? ${conf}%`}><input type="range" min={0} max={100} step={5} value={conf} onChange={e => setConf(Number(e.target.value))} /></Field>
            {!it && template?.predictions?.length ? <div className="chips">{template.predictions.map(p => <button key={p} type="button" className="chip" onClick={() => setBody(p)}>{p}</button>)}</div> : null}
          </>
        )}
        {kind === 'question' && (
          <>
            <Field label="The question" help="Whoever opens it can answer it there and then."><input value={body} onChange={e => setBody(e.target.value)} placeholder="Do you still live in the same place?" autoFocus maxLength={500} /></Field>
            {!it && template?.questions?.length ? <div className="chips">{template.questions.map(p => <button key={p} type="button" className="chip" onClick={() => setBody(p)}>{p}</button>)}</div> : null}
          </>
        )}
        {kind === 'goal' && <Field label="The goal" help="At the opening you mark whether you did it — and can turn it into a task if you still want to."><input value={body} onChange={e => setBody(e.target.value)} placeholder="Run a 10 km race" autoFocus maxLength={500} /></Field>}
        {kind === 'list' && (
          <>
            <Field label="Title"><input value={title} onChange={e => setTitle(e.target.value)} placeholder="Right now" maxLength={200} /></Field>
            {!it && template?.lists?.length ? <div className="chips">{template.lists.map(l => <button key={l.title} type="button" className="chip" onClick={() => { setTitle(l.title); setEntries(l.labels.map(x => ({ label: x, value: '' }))) }}>{l.title}</button>)}</div> : null}
            <div className="entries">
              {entries.map((e, i) => (
                <div key={i} className="entry-row">
                  <input value={e.label} onChange={ev => setEntries(x => x.map((y, j) => j === i ? { ...y, label: ev.target.value } : y))} placeholder="Favourite song" />
                  <input value={e.value} onChange={ev => setEntries(x => x.map((y, j) => j === i ? { ...y, value: ev.target.value } : y))} placeholder="…" />
                  <button type="button" className="btn ghost icon-only sm" aria-label="Remove the line" onClick={() => setEntries(x => x.filter((_, j) => j !== i))}><Icon name="x" size={14} /></button>
                </div>
              ))}
              <button type="button" className="btn sm" onClick={() => setEntries(x => [...x, { label: '', value: '' }])}><Icon name="plus" size={14} />A line</button>
            </div>
          </>
        )}
        {kind === 'link' && (
          <>
            <Field label="Link"><input value={url} onChange={e => setUrl(e.target.value)} placeholder="https://…" type="url" autoFocus /></Field>
            <Field label="Title"><input value={title} onChange={e => setTitle(e.target.value)} maxLength={200} /></Field>
            <Field label="Why it is in here"><input value={body} onChange={e => setBody(e.target.value)} maxLength={2000} /></Field>
          </>
        )}
        {kind === 'audio' && !it && <Recorder onFile={f => setFile(f)} />}
        {needsFile && (
          <>
            {!it && (
              <Field label={kind === 'audio' ? 'Or a recording you have' : kind === 'photo' ? 'The picture' : kind === 'video' ? 'The video' : 'The file'} help={kind === 'video' ? 'Up to 25 MB — a short clip.' : 'Up to 25 MB.'}>
                <input type="file" accept={kind === 'photo' ? 'image/*' : kind === 'audio' ? 'audio/*' : kind === 'video' ? 'video/*' : undefined} onChange={e => { setFile(e.target.files?.[0] || null); setUploadId(null) }} />
              </Field>
            )}
            {file && kind === 'photo' && <PreviewImage file={file} />}
            <Field label="Title (optional)"><input value={title} onChange={e => setTitle(e.target.value)} maxLength={200} /></Field>
            <Field label={kind === 'audio' ? 'What it is (optional)' : 'A line about it (optional)'}><input value={body} onChange={e => setBody(e.target.value)} maxLength={2000} /></Field>
          </>
        )}
      </div>
    </Modal>
  )
}

function PreviewImage({ file }: { file: File }) {
  const [src, setSrc] = useState('')
  useEffect(() => { const u = URL.createObjectURL(file); setSrc(u); return () => URL.revokeObjectURL(u) }, [file])
  return src ? <img src={src} alt="" className="preview-img" /> : null
}

/** Record in the browser (the microphone is asked for only when you press Record). */
function Recorder({ onFile }: { onFile: (f: File | null) => void }) {
  const [state, setState] = useState<'idle' | 'rec' | 'done' | 'denied'>('idle')
  const [secs, setSecs] = useState(0)
  const [src, setSrc] = useState('')
  const rec = useRef<MediaRecorder | null>(null)
  const timer = useRef(0)
  useEffect(() => () => { window.clearInterval(timer.current); rec.current?.stream.getTracks().forEach(t => t.stop()) }, [])
  const start = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const type = ['audio/webm;codecs=opus', 'audio/mp4', 'audio/webm'].find(t => MediaRecorder.isTypeSupported(t)) || ''
      const mr = new MediaRecorder(stream, type ? { mimeType: type } : undefined)
      const chunks: Blob[] = []
      mr.ondataavailable = e => { if (e.data.size) chunks.push(e.data) }
      mr.onstop = () => {
        stream.getTracks().forEach(t => t.stop())
        const mime = (mr.mimeType || 'audio/webm').split(';')[0]
        const blob = new Blob(chunks, { type: mime })
        const f = new File([blob], `recording.${mime.includes('mp4') ? 'm4a' : 'webm'}`, { type: mime })
        setSrc(URL.createObjectURL(blob)); setState('done'); onFile(f)
      }
      rec.current = mr
      mr.start(1000)
      setSecs(0); setState('rec'); onFile(null)
      timer.current = window.setInterval(() => setSecs(s => { if (s >= 600) { mr.stop(); window.clearInterval(timer.current) } return s + 1 }), 1000)
    } catch { setState('denied') }
  }
  const stop = () => { window.clearInterval(timer.current); rec.current?.stop() }
  return (
    <div className="recorder">
      {state === 'rec' ? (
        <button type="button" className="btn danger" onClick={stop}><Icon name="stop" size={15} />Stop · {Math.floor(secs / 60)}:{String(secs % 60).padStart(2, '0')}</button>
      ) : (
        <button type="button" className="btn" onClick={start}><Icon name="mic" size={15} />{state === 'done' ? 'Record again' : 'Record'}</button>
      )}
      {state === 'rec' && <span className="rec-dot" aria-hidden />}
      {state === 'done' && src && <audio controls src={src} />}
      {state === 'denied' && <span className="muted small">The browser did not allow the microphone. You can still pick a recording below.</span>}
    </div>
  )
}
