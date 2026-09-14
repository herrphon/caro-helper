import { useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { api, type Folder, type GridKind, type SheetAlias, type Status, type Workspace } from "@/lib/api"

type Props = {
  status: Status | null
  sheets: SheetAlias[]
  onChanged: () => void
}

export function SettingsPage({ status, sheets, onChanged }: Props) {
  return (
    <div className="mx-auto flex max-w-3xl flex-col gap-6">
      <TokenCard status={status} onChanged={onChanged} />
      <SheetsCard sheets={sheets} canBrowse={!!status?.tokenValid} onChanged={onChanged} />
      {status && (
        <p className="text-muted-foreground text-xs">Config file: {status.configPath}</p>
      )}
    </div>
  )
}

function TokenCard({ status, onChanged }: { status: Status | null; onChanged: () => void }) {
  const [token, setToken] = useState("")
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState<string | null>(null)

  async function save() {
    setBusy(true)
    setMsg(null)
    try {
      const r = await api.setToken(token)
      setMsg(`Token accepted for ${r.user}`)
      setToken("")
      onChanged()
    } catch (e) {
      setMsg((e as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          Smartsheet access
          {status?.tokenValid ? (
            <Badge>connected as {status.user}</Badge>
          ) : status?.hasToken ? (
            <Badge variant="destructive">token invalid</Badge>
          ) : (
            <Badge variant="secondary">not configured</Badge>
          )}
        </CardTitle>
        <CardDescription>
          Generate a token in Smartsheet under Account &rarr; Personal Settings &rarr; API Access and paste it here.
          It is stored encrypted for this Windows user only.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {status?.tokenError && <p className="text-destructive text-sm">{status.tokenError}</p>}
        <div className="flex gap-2">
          <Input
            type="password"
            placeholder="paste token"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && token && save()}
          />
          <Button onClick={save} disabled={busy || !token.trim()}>
            {busy ? "Checking..." : "Save"}
          </Button>
        </div>
        {msg && <p className="text-sm">{msg}</p>}
      </CardContent>
    </Card>
  )
}

function SheetsCard({
  sheets,
  canBrowse,
  onChanged,
}: {
  sheets: SheetAlias[]
  canBrowse: boolean
  onChanged: () => void
}) {
  const [draft, setDraft] = useState<SheetAlias[]>(sheets)
  const [tree, setTree] = useState<Workspace[] | null>(null)
  const [loadingTree, setLoadingTree] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => setDraft(sheets), [sheets])

  const dirty = JSON.stringify(draft) !== JSON.stringify(sheets)

  async function loadTree() {
    setLoadingTree(true)
    setErr(null)
    try {
      setTree(await api.browse())
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setLoadingTree(false)
    }
  }

  function add(ref: { id: number; name: string }, kind: GridKind) {
    if (draft.some((s) => s.sheetId === ref.id)) return
    setDraft([...draft, { alias: ref.name, kind, sheetId: ref.id }])
  }

  async function save() {
    setSaving(true)
    setErr(null)
    try {
      await api.setSheets(draft)
      onChanged()
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Sheets</CardTitle>
        <CardDescription>
          The sheets shown in the sidebar. Give each a short name; pick them from your workspaces below.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {draft.length === 0 && <p className="text-muted-foreground text-sm">No sheets yet.</p>}
        {draft.map((s, i) => (
          <div key={s.sheetId} className="flex items-center gap-2">
            <Input
              value={s.alias}
              onChange={(e) => {
                const next = [...draft]
                next[i] = { ...s, alias: e.target.value }
                setDraft(next)
              }}
            />
            <Badge variant="outline" className="shrink-0">{s.kind}</Badge>
            <code className="text-muted-foreground shrink-0 text-xs">{s.sheetId}</code>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setDraft(draft.filter((_, j) => j !== i))}
            >
              Remove
            </Button>
          </div>
        ))}
        <div className="flex gap-2">
          <Button onClick={save} disabled={!dirty || saving}>
            {saving ? "Saving..." : "Save sheets"}
          </Button>
          <Button variant="outline" onClick={loadTree} disabled={!canBrowse || loadingTree}>
            {loadingTree ? "Loading workspaces..." : tree ? "Reload workspaces" : "Browse workspaces"}
          </Button>
        </div>
        {err && <p className="text-destructive text-sm">{err}</p>}
        {tree && (
          <div className="rounded-md border p-3 text-sm">
            {tree.length === 0 && <p className="text-muted-foreground">No workspaces visible.</p>}
            {tree.map((ws) => (
              <FolderNode key={ws.id} node={ws} depth={0} onPick={add} chosen={draft} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function FolderNode({
  node,
  depth,
  onPick,
  chosen,
}: {
  node: Folder
  depth: number
  onPick: (r: { id: number; name: string }, kind: GridKind) => void
  chosen: SheetAlias[]
}) {
  const [open, setOpen] = useState(false)
  const hasChildren =
    (node.sheets?.length ?? 0) + (node.reports?.length ?? 0) + (node.folders?.length ?? 0) > 0
  const items: { ref: { id: number; name: string }; kind: GridKind }[] = [
    ...(node.sheets ?? []).map((ref) => ({ ref, kind: "sheet" as GridKind })),
    ...(node.reports ?? []).map((ref) => ({ ref, kind: "report" as GridKind })),
  ]
  return (
    <div style={{ paddingLeft: depth * 12 }}>
      <button
        type="button"
        className="hover:bg-accent flex w-full items-center gap-1 rounded px-1 py-0.5 text-left font-medium"
        onClick={() => setOpen(!open)}
      >
        <span className="text-muted-foreground w-3 text-xs">{hasChildren ? (open ? "v" : ">") : ""}</span>
        {node.name}
      </button>
      {open && (
        <div>
          {items.map(({ ref, kind }) => {
            const picked = chosen.some((c) => c.sheetId === ref.id)
            return (
              <div key={ref.id} className="flex items-center justify-between gap-2 py-0.5 pl-5">
                <span className={picked ? "text-muted-foreground" : ""}>
                  {ref.name}{" "}
                  <span className="text-muted-foreground text-xs">{kind}</span>
                </span>
                <Button size="sm" variant="ghost" disabled={picked} onClick={() => onPick(ref, kind)}>
                  {picked ? "added" : "add"}
                </Button>
              </div>
            )
          })}
          {node.folders?.map((f) => (
            <FolderNode key={f.id} node={f} depth={depth + 1} onPick={onPick} chosen={chosen} />
          ))}
        </div>
      )}
    </div>
  )
}
