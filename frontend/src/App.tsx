import { useCallback, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
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
      <aside className="bg-muted/40 flex w-56 shrink-0 flex-col border-r">
        <div className="px-4 py-3">
          <div className="text-base font-semibold">Caro Helper</div>
          <div className="text-muted-foreground text-xs">
            {status?.tokenValid ? status.user : "not connected"}
          </div>
        </div>
        <Separator />
        <nav className="flex flex-1 flex-col gap-0.5 overflow-auto p-2">
          <div className="text-muted-foreground px-2 pb-1 pt-2 text-xs font-medium uppercase">
            Sheets
          </div>
          {sheets.length === 0 && (
            <div className="text-muted-foreground px-2 text-xs">none configured</div>
          )}
          {sheets.map((s) => (
            <Button
              key={s.sheetId}
              variant={current?.sheetId === s.sheetId ? "secondary" : "ghost"}
              className="justify-start"
              onClick={() => (window.location.hash = `#/sheet/${s.sheetId}`)}
            >
              <span className="truncate">{s.alias}</span>
            </Button>
          ))}
        </nav>
        <Separator />
        <div className="flex flex-col gap-0.5 p-2">
          <Button
            variant={route.page === "settings" ? "secondary" : "ghost"}
            className="w-full justify-start"
            onClick={() => (window.location.hash = "#/settings")}
          >
            Settings
          </Button>
          <Button variant="ghost" className="w-full justify-start" onClick={toggleTheme}>
            {theme === "dark" ? "Light mode" : "Dark mode"}
          </Button>
        </div>
      </aside>

      <main className="min-w-0 flex-1 overflow-auto p-6">
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
