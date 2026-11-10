// Pictures are shrunk in the browser before they go up (2000 px, and a
// 480 px thumbnail); other files go as they are.
import { ApiError, V1 } from './api'

async function toCanvas(file: Blob, max: number): Promise<HTMLCanvasElement> {
  const url = URL.createObjectURL(file)
  try {
    const img = await new Promise<HTMLImageElement>((res, rej) => { const i = new Image(); i.onload = () => res(i); i.onerror = rej; i.src = url })
    const scale = Math.min(1, max / Math.max(img.naturalWidth, img.naturalHeight))
    const c = document.createElement('canvas')
    c.width = Math.round(img.naturalWidth * scale)
    c.height = Math.round(img.naturalHeight * scale)
    c.getContext('2d')!.drawImage(img, 0, 0, c.width, c.height)
    return c
  } finally { URL.revokeObjectURL(url) }
}

const blobOf = (c: HTMLCanvasElement, type: string, q = 0.86) => new Promise<Blob>((res, rej) => c.toBlob(b => b ? res(b) : rej(new Error('could not read the picture')), type, q))

export interface Uploaded { url: string; markdown: string; upload: { id: number; name: string; mime: string } }

/** A picture shrunk for a form of its own (the give page): the big one and its thumbnail. */
export async function shrink(file: File): Promise<{ big: Blob; thumb: Blob; w: number; h: number; name: string } | null> {
  if (!/^image\/(jpeg|png|webp)$/.test(file.type)) return null
  const big = await toCanvas(file, 2000)
  const small = await toCanvas(file, 480)
  return { big: await blobOf(big, 'image/jpeg'), thumb: await blobOf(small, 'image/jpeg', 0.8), w: big.width, h: big.height, name: file.name.replace(/\.(png|webp)$/i, '.jpg') }
}

export async function uploadFile(file: File, capsule: number, url = `${V1}/uploads`): Promise<Uploaded> {
  const fd = new FormData()
  fd.append('capsule', String(capsule))
  let name = file.name || 'file'
  if (/^image\/(jpeg|png|webp)$/.test(file.type) && file.size > 300 * 1024) {
    const big = await toCanvas(file, 2000)
    const small = await toCanvas(file, 480)
    const type = file.type === 'image/png' && file.size < 2 * 1024 * 1024 ? 'image/png' : 'image/jpeg'
    if (type === 'image/jpeg') name = name.replace(/\.(png|webp)$/i, '.jpg')
    fd.append('file', await blobOf(big, type), name)
    fd.append('thumb', await blobOf(small, 'image/jpeg', 0.8), 'thumb.jpg')
    fd.append('width', String(big.width))
    fd.append('height', String(big.height))
  } else {
    fd.append('file', file, name)
  }
  const res = await fetch(url, { method: 'POST', body: fd, credentials: 'same-origin' })
  const j = await res.json().catch(() => ({}))
  if (!res.ok) throw new ApiError(res.status, j.error || res.statusText, j)
  return j as Uploaded
}
