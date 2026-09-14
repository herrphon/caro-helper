import { useEffect, useMemo, useState } from "react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { api, type Column, type RowView, type SheetAlias, type SheetView } from "@/lib/api"

type Sort = { colId: string; dir: "asc" | "desc" } | null

export function SheetPage({ alias }: { alias: SheetAlias }) {
  const [sheet, setSheet] = useState<SheetView | null>(null)
  const [err, setErr] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [query, setQuery] = useState("")
  const [sort, setSort] = useState<Sort>(null)
  const [copied, setCopied] = useState(false)

  async function load() {
    setLoading(true)
    setErr(null)
    try {
      setSheet(await api.sheet(alias.sheetId))
    } catch (e) {
      setErr((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    setSheet(null)
    setQuery("")
    setSort(null)
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [alias.sheetId])

  const rows = useMemo(() => {
    if (!sheet) return []
    const terms = query.toLowerCase().split(/\s+/).filter(Boolean)
    let out = sheet.rows
    if (terms.length) {
      out = out.filter((r) => {
        const hay = Object.values(r.cells).join(" \u0000 ").toLowerCase()
        return terms.every((t) => hay.includes(t))
      })
    }
    if (sort) {
      const { colId, dir } = sort
      out = [...out].sort((a, b) => {
        const av = a.cells[colId] ?? ""
        const bv = b.cells[colId] ?? ""
        const an = Number(av), bn = Number(bv)
        const cmp =
          av !== "" && bv !== "" && !isNaN(an) && !isNaN(bn)
            ? an - bn
            : av.localeCompare(bv, undefined, { numeric: true, sensitivity: "base" })
        return dir === "asc" ? cmp : -cmp
      })
    }
    return out
  }, [sheet, query, sort])

  function toggleSort(colId: string) {
    setSort((s) =>
      s?.colId !== colId ? { colId, dir: "asc" } : s.dir === "asc" ? { colId, dir: "desc" } : null,
    )
  }

  async function copyTSV() {
    if (!sheet) return
    const cols = sheet.columns
    const lines = [
      cols.map((c) => c.title).join("\t"),
      ...rows.map((r) => cols.map((c) => (r.cells[String(c.id)] ?? "").replace(/\t|\n/g, " ")).join("\t")),
    ]
    await navigator.clipboard.writeText(lines.join("\n"))
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">{alias.alias}</h1>
        {sheet && (
          <>
            <Badge variant="secondary">
              {rows.length}
              {rows.length !== sheet.rows.length && ` / ${sheet.rows.length}`} rows
            </Badge>
            <a
              className="text-muted-foreground text-xs underline-offset-2 hover:underline"
              href={sheet.permalink}
              target="_blank"
              rel="noreferrer"
            >
              open in Smartsheet
            </a>
          </>
        )}
        <div className="ml-auto flex items-center gap-2">
          <Input
            className="w-72"
            placeholder="Search all columns... (space = AND)"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
          />
          <Button variant="outline" size="sm" onClick={copyTSV} disabled={!sheet || rows.length === 0}>
            {copied ? "Copied" : "Copy for Excel"}
          </Button>
          <Button variant="outline" size="sm" onClick={load} disabled={loading}>
            {loading ? "Loading..." : "Refresh"}
          </Button>
        </div>
      </div>

      {err && <p className="text-destructive text-sm">{err}</p>}
      {loading && !sheet && <p className="text-muted-foreground text-sm">Loading sheet...</p>}

      {sheet && (
        <div className="min-h-0 flex-1 overflow-auto rounded-md border">
          <Table>
            <TableHeader className="bg-background sticky top-0 z-10">
              <TableRow>
                <TableHead className="w-10 text-right">#</TableHead>
                {sheet.columns.map((c) => (
                  <TableHead
                    key={c.id}
                    className="cursor-pointer select-none whitespace-nowrap"
                    onClick={() => toggleSort(String(c.id))}
                    title={c.type}
                  >
                    {c.title}
                    {sort?.colId === String(c.id) && (sort.dir === "asc" ? " \u2191" : " \u2193")}
                  </TableHead>
                ))}
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((r) => (
                <Row key={r.id} row={r} columns={sheet.columns} />
              ))}
              {rows.length === 0 && (
                <TableRow>
                  <TableCell colSpan={sheet.columns.length + 1} className="text-muted-foreground text-center">
                    No rows match.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}

function Row({ row, columns }: { row: RowView; columns: Column[] }) {
  return (
    <TableRow>
      <TableCell className="text-muted-foreground text-right text-xs">{row.rowNumber}</TableCell>
      {columns.map((c) => (
        <TableCell key={c.id} className="max-w-xs truncate" title={row.cells[String(c.id)]}>
          {row.cells[String(c.id)] ?? ""}
        </TableCell>
      ))}
    </TableRow>
  )
}
