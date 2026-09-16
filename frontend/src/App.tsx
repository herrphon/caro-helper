import { useCallback, useEffect, useState, type ReactNode } from "react"
import { FileSpreadsheet, Grid3x3, Globe, Menu, Moon, Settings, ShoppingCart, Sun, Table2 } from "lucide-react"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { api, type SheetAlias, type Status } from "@/lib/api"
import { SettingsPage } from "@/pages/SettingsPage"
import { SheetPage } from "@/pages/SheetPage"

// --- data sources -------------------------------------------------------

type SourceId = "smartsheet" | "ariba" | "excel" | "webapp1" | "webapp2"

const SOURCES: { id: SourceId; label: string; icon: typeof Grid3x3; ready: boolean }[] = [
  { id: "smartsheet", label: "Smartsheet", icon: Grid3x3, ready: true },
  { id: "ariba", label: "SAP Ariba", icon: ShoppingCart, ready: false },
  { id: "excel", label: "Excel", icon: FileSpreadsheet, ready: false },
  { id: "webapp1", label: "Web App 1", icon: Globe, ready: false },
  { id: "webapp2", label: "Web App 2", icon: Globe, ready: false },
]

// --- routing ------------------------------------------------------------

type Route =
  | { page: "settings" }
  | { page: "source"; source: SourceId; sheetId?: number }

function parseHash(): Route {
  const h = window.location.hash
  if (h === "#/settings") return { page: "settings" }
  const m = h.match(/^#\/([a-z0-9]+)(?:\/sheet\/(\d+))?$/)
  if (m && SOURCES.some((s) => s.id === m[1])) {
    return { page: "source", source: m[1] as SourceId, sheetId: m[2] ? Number(m[2]) : undefined }
  }
  return { page: "source", source: "smartsheet" }
}

function go(hash: string) {
  window.location.hash = hash
}

// --- theme --------------------------------------------------------------

type Theme = "light" | "dark"

function useTheme(): [Theme, () => void] {
  const [theme, setTheme] = useState<Theme>(() => {
    const saved = localStorage.getItem("theme")
    if (saved === "light" || saved === "dark") return saved
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
  })
  useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark")
    localStorage.setItem("theme", theme)
  }, [theme])
  return [theme, () => setTheme((t) => (t === "dark" ? "light" : "dark"))]
}

// --- app ----------------------------------------------------------------

export default function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [sheets, setSheets] = useState<SheetAlias[]>([])
  const [route, setRoute] = useState<Route>(parseHash)
  const [loadErr, setLoadErr] = useState<string | null>(null)
  const [theme, toggleTheme] = useTheme()
  const [navOpen, setNavOpen] = useState(() => localStorage.getItem("navOpen") !== "0")
  useEffect(() => localStorage.setItem("navOpen", navOpen ? "1" : "0"), [navOpen])

  const refresh = useCallback(async () => {
    try {
      const [st, sh] = await Promise.all([api.status(), api.sheets()])
      setStatus(st)
      setSheets(sh)
      setLoadErr(null)
    } catch (e) {
      setLoadErr((e as Error).message)
    }
  }, [])

  useEffect(() => {
    void refresh()
    const onHash = () => setRoute(parseHash())
    window.addEventListener("hashchange", onHash)
    return () => window.removeEventListener("hashchange", onHash)
  }, [refresh])

  // first run without token -> settings; otherwise land on the first sheet
  useEffect(() => {
    if (!status || window.location.hash) return
    if (!status.tokenValid) go("#/settings")
    else if (sheets.length > 0) go(`#/smartsheet/sheet/${sheets[0].sheetId}`)
  }, [status, sheets])

  const activeSource: SourceId = route.page === "source" ? route.source : "smartsheet"
  const current =
    route.page === "source" && route.sheetId !== undefined
      ? sheets.find((s) => s.sheetId === route.sheetId)
      : undefined

  return (
    <div className="bg-background flex h-screen text-sm">
      {navOpen && (
        <>
          <SourceRail active={activeSource} settingsActive={route.page === "settings"} />
          <SheetPanel
            source={activeSource}
            sheets={sheets}
            currentId={current?.sheetId}
            hidden={route.page === "settings"}
          />
        </>
      )}

      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar
          status={status}
          theme={theme}
          onToggleTheme={toggleTheme}
          onToggleNav={() => setNavOpen((v) => !v)}
          navOpen={navOpen}
        />

        <main className="bg-muted/40 min-w-0 flex-1 overflow-auto p-4">
          {loadErr && <p className="text-destructive mb-4">Cannot reach Caro Helper: {loadErr}</p>}
          {route.page === "settings" ? (
            <SettingsPage status={status} sheets={sheets} onChanged={refresh} />
          ) : activeSource !== "smartsheet" ? (
            <ComingSoon source={activeSource} />
          ) : current ? (
            <SheetPage alias={current} />
          ) : (
            <EmptyState hasSheets={sheets.length > 0} />
          )}
        </main>
      </div>
    </div>
  )
}

// --- pieces -------------------------------------------------------------

function TopBar({
  status,
  theme,
  onToggleTheme,
  onToggleNav,
  navOpen,
}: {
  status: Status | null
  theme: Theme
  onToggleTheme: () => void
  onToggleNav: () => void
  navOpen: boolean
}) {
  const initials = (status?.user ?? "?").slice(0, 2).toUpperCase()
  return (
    <header className="bg-sidebar text-sidebar-foreground flex h-12 shrink-0 items-center px-2">
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={onToggleNav}
          title={navOpen ? "Hide navigation" : "Show navigation"}
          className="hover:bg-sidebar-accent flex size-9 items-center justify-center rounded-sm text-white"
        >
          <Menu className={"size-5 transition-transform duration-200 " + (navOpen ? "" : "scale-x-[-1]")} />
        </button>
        <span className="font-semibold text-white">Caro Helper</span>
      </div>
      <div className="ml-auto">
        <DropdownMenu>
          <DropdownMenuTrigger
            className="bg-sidebar-primary hover:ring-sidebar-ring flex size-8 items-center justify-center rounded-full text-xs font-semibold text-white hover:ring-2"
            title={status?.user || "not connected"}
          >
            {initials}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64">
            <DropdownMenuGroup>
              <DropdownMenuLabel className="truncate font-normal">
                {status?.tokenValid ? status.user : "Not connected to Smartsheet"}
              </DropdownMenuLabel>
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => go("#/settings")}>
              <Settings className="size-4" /> Settings
            </DropdownMenuItem>
            <DropdownMenuItem onClick={onToggleTheme}>
              {theme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
              {theme === "dark" ? "Light mode" : "Dark mode"}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}

function SourceRail({ active, settingsActive }: { active: SourceId; settingsActive: boolean }) {
  return (
    <nav className="bg-sidebar flex w-12 shrink-0 flex-col items-center gap-1 border-r border-white/10 py-2">
      {SOURCES.map((s) => {
        const Icon = s.icon
        const isActive = !settingsActive && s.id === active
        return (
          <button
            key={s.id}
            type="button"
            title={s.ready ? s.label : `${s.label} (coming soon)`}
            onClick={() => go(`#/${s.id}`)}
            className={
              "relative flex size-10 items-center justify-center rounded-sm transition-colors " +
              (isActive
                ? "bg-sidebar-accent text-white"
                : "text-sidebar-foreground/70 hover:bg-sidebar-accent/60 hover:text-white") +
              (s.ready ? "" : " opacity-50")
            }
          >
            {isActive && <span className="bg-sidebar-primary absolute left-0 top-1 h-8 w-0.5 rounded-r" />}
            <Icon className="size-5" />
          </button>
        )
      })}
    </nav>
  )
}

function SheetPanel({
  source,
  sheets,
  currentId,
  hidden,
}: {
  source: SourceId
  sheets: SheetAlias[]
  currentId?: number
  hidden: boolean
}) {
  if (hidden) return null
  const src = SOURCES.find((s) => s.id === source)!
  return (
    <aside className="text-sidebar-foreground flex w-56 shrink-0 flex-col" style={{ backgroundColor: "var(--sidebar-panel)" }}>
      <div className="px-4 py-3 text-sm font-semibold text-white">{src.label}</div>
      <div className="flex-1 overflow-auto px-2 pb-2">
        {source !== "smartsheet" && (
          <p className="text-sidebar-foreground/60 px-2 text-xs">Not connected yet.</p>
        )}
        {source === "smartsheet" && sheets.length === 0 && (
          <p className="text-sidebar-foreground/60 px-2 text-xs">
            No sheets yet. Add them under Settings.
          </p>
        )}
        {source === "smartsheet" &&
          sheets.map((s) => (
            <PanelItem
              key={s.sheetId}
              active={s.sheetId === currentId}
              icon={<Table2 className="size-3.5 shrink-0 opacity-70" />}
              onClick={() => go(`#/smartsheet/sheet/${s.sheetId}`)}
            >
              {s.alias}
            </PanelItem>
          ))}
      </div>
    </aside>
  )
}

function PanelItem({
  active,
  icon,
  onClick,
  children,
}: {
  active: boolean
  icon?: ReactNode
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        "flex w-full items-center gap-2 rounded-sm border-l-2 px-3 py-1.5 text-left text-sm transition-colors " +
        (active
          ? "border-sidebar-primary bg-sidebar-accent text-white"
          : "hover:bg-sidebar-accent/60 border-transparent")
      }
    >
      {icon}
      <span className="truncate">{children}</span>
    </button>
  )
}

function ComingSoon({ source }: { source: SourceId }) {
  const src = SOURCES.find((s) => s.id === source)!
  return (
    <div className="text-muted-foreground flex h-full flex-col items-center justify-center gap-2">
      <src.icon className="size-10 opacity-40" />
      <p className="text-base font-medium">{src.label}</p>
      <p className="text-xs">This data source is not connected yet.</p>
    </div>
  )
}

function EmptyState({ hasSheets }: { hasSheets: boolean }) {
  return (
    <div className="text-muted-foreground flex h-full flex-col items-center justify-center gap-2">
      <Grid3x3 className="size-10 opacity-40" />
      <p className="text-xs">{hasSheets ? "Pick a sheet on the left." : "Add sheets under Settings (avatar menu, top right)."}</p>
    </div>
  )
}
