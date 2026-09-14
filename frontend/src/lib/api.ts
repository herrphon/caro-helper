export type Status = {
  hasToken: boolean
  tokenValid: boolean
  user: string
  configPath: string
  tokenError?: string
}

export type SheetAlias = { alias: string; sheetId: number }

export type Column = {
  id: number
  index: number
  title: string
  type: string
  primary?: boolean
  options?: string[]
}

export type RowView = {
  id: number
  rowNumber: number
  cells: Record<string, string>
}

export type SheetView = {
  id: number
  name: string
  permalink: string
  modifiedAt: string
  totalRows: number
  columns: Column[]
  rows: RowView[]
}

export type ItemRef = { id: number; name: string; permalink?: string }
export type Folder = ItemRef & {
  sheets?: ItemRef[]
  folders?: Folder[]
  reports?: ItemRef[]
}
export type Workspace = Folder

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(url, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
  })
  const body = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new Error((body as { error?: string }).error ?? `HTTP ${res.status}`)
  }
  return body as T
}

export const api = {
  status: () => request<Status>("/api/status"),
  setToken: (token: string) =>
    request<{ ok: boolean; user: string }>("/api/token", {
      method: "PUT",
      body: JSON.stringify({ token }),
    }),
  sheets: () => request<SheetAlias[]>("/api/sheets"),
  setSheets: (sheets: SheetAlias[]) =>
    request<SheetAlias[]>("/api/sheets", { method: "PUT", body: JSON.stringify(sheets) }),
  sheet: (id: number) => request<SheetView>(`/api/sheets/${id}`),
  browse: () => request<Workspace[]>("/api/browse"),
}
