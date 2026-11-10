// The app shell: sidebar, top bar, search palette, shortcuts — the Pocket Calendar layout.
import { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { DataProvider } from '../lib/data'
import { Icon } from './icons'
import { Sidebar } from './Sidebar'
import { CommandPalette, type PaletteCommand } from './CommandPalette'
import { ShortcutsSheet } from './ShortcutsSheet'
import { Avatar, type MenuItem, MenuButton, modalOpen, useIsMobile, useMediaQuery } from './ui'
import { AskButton } from './AskButton'

export { announceChange, useOnChange } from '../lib/data'

const SIDEBAR_KEY = 'pcsl-sidebar'

/** The page's own name for the top bar breadcrumb. */
const CrumbCtx = createContext<(t: string, to?: string) => void>(() => {})
export function useCrumb(title: string | undefined, parent?: string) {
  const set = useContext(CrumbCtx)
  useEffect(() => { set(title || '', parent); return () => set('') }, [title, parent, set])
}

/** Start a capsule from anywhere (a page's button). */
export const openCreate = (template?: string) => window.dispatchEvent(new CustomEvent('pcsl:create', { detail: { template } }))

export default function Layout() {
  return <DataProvider><Shell /></DataProvider>
}

function Shell() {
  const navigate = useNavigate()
  const loc = useLocation()
  const mobile = useIsMobile()
  const narrow = useMediaQuery('(max-width: 1099px)')
  const drawer = mobile || narrow
  const [sidebarOpen, setSidebarOpen] = useState(() => !drawer && localStorage.getItem(SIDEBAR_KEY) !== '0')
  useEffect(() => { setSidebarOpen(!drawer && localStorage.getItem(SIDEBAR_KEY) !== '0') }, [drawer])
  const drawerOpenedAt = useRef(0)
  const toggleSidebar = () => setSidebarOpen(o => { drawerOpenedAt.current = performance.now(); const n = !o; if (!drawer) localStorage.setItem(SIDEBAR_KEY, n ? '1' : '0'); return n })
  const closeDrawer = () => { if (drawer) setSidebarOpen(false) }
  useEffect(() => { document.querySelector('.content')?.scrollTo({ top: 0 }) }, [loc.pathname])

  const [crumb, setCrumbRaw] = useState<{ t: string; to?: string }>({ t: '' })
  const setCrumb = useCallback((t: string, to?: string) => setCrumbRaw({ t, to }), [])

  const [palette, setPalette] = useState(false)
  const [shortcuts, setShortcuts] = useState(false)

  const create = (template?: string) => {
    closeDrawer()
    navigate(template ? `/new?template=${template}` : '/new')
  }
  const createRef = useRef(create)
  createRef.current = create

  useEffect(() => {
    const open = () => setShortcuts(true)
    const make = (e: Event) => createRef.current((e as CustomEvent<{ template?: string }>).detail.template)
    window.addEventListener('pcsl:shortcuts', open)
    window.addEventListener('pcsl:create', make)
    return () => { window.removeEventListener('pcsl:shortcuts', open); window.removeEventListener('pcsl:create', make) }
  }, [])
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') { e.preventDefault(); setPalette(p => !p); return }
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (t.closest('input, textarea, select, [contenteditable="true"]') || modalOpen() || document.querySelector('.menu, .popover, .sel-pop')) return
      const go = (to: string) => { e.preventDefault(); navigate(to) }
      switch (e.key) {
        case '/': e.preventDefault(); setPalette(true); break
        case '?': e.preventDefault(); setShortcuts(true); break
        case 'n': e.preventDefault(); createRef.current(); break
        case 'h': go('/'); break
        case 'r': go('/?show=ready'); break
        case 'p': go('/predictions'); break
        case 'y': go('/yearly'); break
        case ',': go('/settings'); break
        case '[': e.preventDefault(); toggleSidebar(); break
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const commands: PaletteCommand[] = [
    { id: 'new', label: 'New capsule', icon: 'plus', hint: 'N', keywords: 'make start time capsule letter', run: () => create() },
    { id: 'new-self', label: 'New: letter to future me', icon: 'feather', keywords: 'self year', run: () => create('self') },
    { id: 'new-child', label: 'New: for my child (18th birthday)', icon: 'gift', keywords: 'baby kid son daughter', run: () => create('child') },
    { id: 'new-family', label: 'New: family capsule', icon: 'users', keywords: 'together everyone', run: () => create('family') },
    { id: 'new-year', label: 'New: New Year\'s capsule', icon: 'sparkles', keywords: 'resolutions goals', run: () => create('newyear') },
    { id: 'home', label: 'All capsules', icon: 'hourglass', hint: 'H', run: () => navigate('/') },
    { id: 'ready', label: 'Ready to open', icon: 'gift', hint: 'R', keywords: 'open today', run: () => navigate('/?show=ready') },
    { id: 'sealed', label: 'Sealed capsules', icon: 'lock', keywords: 'waiting countdown', run: () => navigate('/?show=sealed') },
    { id: 'opened', label: 'Opened capsules', icon: 'unlock', keywords: 'archive past', run: () => navigate('/?show=opened') },
    { id: 'predictions', label: 'Predictions and goals', icon: 'sparkles', hint: 'P', keywords: 'guesses came true score', run: () => navigate('/predictions') },
    { id: 'yearly', label: 'Yearly capsules', icon: 'repeat', hint: 'Y', keywords: 'every year reminder ritual', run: () => navigate('/yearly') },
    { id: 'settings', label: 'Settings', icon: 'settings', hint: ',', run: () => navigate('/settings') },
    { id: 'apps', label: 'Settings: connected apps', icon: 'link', keywords: 'calendar contacts family email chat photos notes files voice tasks alerts blog weather bookshelf music movement health wealth core', run: () => navigate('/settings/apps') },
    { id: 'notif', label: 'Settings: notifications', icon: 'bell', keywords: 'push reminder', run: () => navigate('/settings/notifications') },
    { id: 'ai-set', label: 'Settings: AI', icon: 'sparkles', keywords: 'pocket ai openai', run: () => navigate('/settings/ai') },
    { id: 'export', label: 'Settings: export and import', icon: 'download', keywords: 'backup json csv', run: () => navigate('/settings/data') },
    { id: 'shortcuts', label: 'Keyboard shortcuts', icon: 'keyboard', hint: '?', run: () => setShortcuts(true) },
  ]

  return (
    <CrumbCtx.Provider value={setCrumb}>
      <div className={`app ${sidebarOpen ? 'with-sidebar' : 'no-sidebar'} ${mobile ? 'is-mobile' : ''} ${drawer ? 'drawer-mode' : ''}`}>
        {drawer && sidebarOpen && <div className="drawer-backdrop" onClick={e => { if (e.timeStamp - drawerOpenedAt.current > 400) setSidebarOpen(false) }} />}
        <div className={`sidebar-wrap ${sidebarOpen ? 'open' : ''}`}>
          <Sidebar onCreate={create} onNavigate={closeDrawer} />
        </div>
        <div className="main">
          <TopBar onMenu={toggleSidebar} onPalette={() => setPalette(true)} mobile={mobile} compact={narrow} sidebarOpen={sidebarOpen} crumb={crumb} />
          <main className="content"><div className="content-inner"><Outlet /></div></main>
        </div>
        {mobile && /^\/(|predictions|yearly)$/.test(loc.pathname) && <button type="button" className="fab" aria-label="New capsule" title="New capsule" onClick={() => create()}><Icon name="plus" size={26} strokeWidth={2} /></button>}
        {palette && <CommandPalette onClose={() => setPalette(false)} commands={commands} />}
        {shortcuts && <ShortcutsSheet onClose={() => setShortcuts(false)} />}
      </div>
    </CrumbCtx.Provider>
  )
}

const SECTION: Record<string, [string, string]> = {
  '': ['Capsules', '/'], c: ['Capsules', '/'], new: ['New capsule', '/'], predictions: ['Predictions', '/predictions'], yearly: ['Yearly capsules', '/yearly'],
}

function TopBar({ onMenu, onPalette, mobile, compact, sidebarOpen, crumb }: { onMenu: () => void; onPalette: () => void; mobile: boolean; compact: boolean; sidebarOpen: boolean; crumb: { t: string; to?: string } }) {
  const { user, themeMode, setThemeMode, status, setUser } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [section, sectionTo] = SECTION[loc.pathname.split('/')[1] || ''] || ['', '/']
  const signOut = async () => { await api.post(`${V1}/auth/logout`).catch(() => {}); setUser(null); navigate('/login') }
  const avatarMenu = (): MenuItem[] => [
    { heading: user?.name || user?.email || '' },
    { label: 'Profile', icon: 'user', onSelect: () => navigate('/settings/profile') },
    { label: 'Settings', icon: 'settings', onSelect: () => navigate('/settings'), hint: ',' },
    { label: 'Keyboard shortcuts', icon: 'keyboard', onSelect: () => window.dispatchEvent(new CustomEvent('pcsl:shortcuts')), hint: '?' },
    'sep',
    { heading: 'Appearance' },
    { label: 'System', checked: themeMode === 'system', onSelect: () => setThemeMode('system') },
    { label: 'Light', checked: themeMode === 'light', onSelect: () => setThemeMode('light') },
    { label: 'Dark', checked: themeMode === 'dark', onSelect: () => setThemeMode('dark') },
    ...(status?.auth_mode === 'local' ? ['sep' as const, { label: 'Sign out', icon: 'log-out', onSelect: signOut }] : []),
  ]
  return (
    <header className="topbar">
      <button type="button" className="btn ghost icon-only" onClick={onMenu} aria-label={sidebarOpen ? 'Hide the sidebar' : 'Show the sidebar'} title="Menu ([)"><Icon name={mobile || compact ? 'menu' : 'sidebar'} size={18} /></button>
      <h1 className="range-title">
        {crumb.t ? <><button type="button" className="crumb" onClick={() => navigate(crumb.to || sectionTo)}>{section}</button><Icon name="chevron-right" size={16} className="crumb-sep" /><span className="ellipsis">{crumb.t}</span></> : section}
      </h1>
      <span className="spacer" />
      <button type="button" className="btn ghost icon-only" onClick={onPalette} aria-label="Search" title="Search (⌘K or /)"><Icon name="search" size={18} /></button>
      <AskButton />
      <MenuButton label="Account" className="avatar-btn" align="right" width={220} items={avatarMenu}>
        <Avatar name={user?.name} email={user?.email} size={30} />
      </MenuButton>
    </header>
  )
}
