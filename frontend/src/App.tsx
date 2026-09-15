import { useCallback, useEffect, useState, type ReactNode } from "react"
import { Separator } from "@/components/ui/separator"
import { api, type SheetAlias, type Status } from "@/lib/api"
import { SettingsPage } from "@/pages/SettingsPage"
import { SheetPage } from "@/pages/SheetPage"

type Route = { page: "settings" } | { page: "sheet"; sheetId: number }

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

function parseHash(): Route {
  const m = window.location.hash.match(/^#\/sheet\/(\d+)$/)
  if (m) return { page: "sheet", sheetId: Number(m[1]) }
  return { page: "settings" }
}

export default function App() {
  const [status, setStatus] = useState<Status | null>(null)
  const [sheets, setSheets] = useState<SheetAlias[]>([])
  const [route, setRoute] = useState<Route>(parseHash)
  const [loadErr, setLoadErr] = useState<string | null>(null)
  const [theme, toggleTheme] = useTheme()

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

  // first run: land on settings; otherwise open first sheet
  useEffect(() => {
    if (status && status.tokenValid && sheets.length > 0 && !window.location.hash) {
      window.location.hash = `#/sheet/${sheets[0].sheetId}`
    }
  }, [status, sheets])

  const current =
    route.page === "sheet" ? sheets.find((s) => s.sheetId === route.sheetId) : undefined

  return (
    <div className="bg-background flex h-screen text-sm">
      <aside className="bg-sidebar text-sidebar-foreground flex w-56 shrink-0 flex-col">
        <div className="flex items-center gap-2 px-4 py-3">
          <span className="bg-sidebar-primary inline-block size-6 shrink-0 rounded-sm" aria-hidden />
          <div className="min-w-0">
            <div className="text-base font-semibold text-white">Caro Helper</div>
            <div className="text-sidebar-foreground/70 truncate text-xs" title={status?.user}>
              {status?.tokenValid ? status.user : "not connected"}
            </div>
          </div>
        </div>
        <Separator className="bg-sidebar-border" />
        <nav className="flex flex-1 flex-col gap-0.5 overflow-auto p-2">
          <div className="text-sidebar-foreground/60 px-2 pb-1 pt-2 text-xs font-medium uppercase">
            Sheets
          </div>
          {sheets.length === 0 && (
            <div className="text-sidebar-foreground/60 px-2 text-xs">none configured</div>
          )}
          {sheets.map((s) => (
            <NavItem
              key={s.sheetId}
              active={current?.sheetId === s.sheetId}
              onClick={() => (window.location.hash = `#/sheet/${s.sheetId}`)}
            >
              {s.alias}
            </NavItem>
          ))}
        </nav>
        <Separator className="bg-sidebar-border" />
        <div className="flex flex-col gap-0.5 p-2">
          <NavItem active={route.page === "settings"} onClick={() => (window.location.hash = "#/settings")}>
            Settings
          </NavItem>
          <NavItem active={false} onClick={toggleTheme}>
            {theme === "dark" ? "Light mode" : "Dark mode"}
          </NavItem>
        </div>
      </aside>

      <main className="bg-muted/40 min-w-0 flex-1 overflow-auto p-4">
        {loadErr && (
          <p className="text-destructive mb-4">Cannot reach Caro Helper: {loadErr}</p>
        )}
        {route.page === "settings" || !current ? (
          <SettingsPage status={status} sheets={sheets} onChanged={refresh} />
        ) : (
          <SheetPage alias={current} />
        )}
      </main>
    </div>
  )
}

function NavItem({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: ReactNode
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        "flex w-full items-center rounded-sm border-l-2 px-3 py-1.5 text-left text-sm transition-colors " +
        (active
          ? "border-sidebar-primary bg-sidebar-accent text-sidebar-accent-foreground"
          : "hover:bg-sidebar-accent/60 border-transparent")
      }
    >
      <span className="truncate">{children}</span>
    </button>
  )
}
