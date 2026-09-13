// Server-side API access: the web app only ever reads the curated layer
// through the Go API. hosting provider wires API_HOST/API_PORT to the api service's
// private domain (see .hosting_provider/hosting_provider.ts).

export function apiURL(): string {
  const host = process.env.API_HOST ?? "localhost";
  const port = process.env.API_PORT ?? "8080";
  return `http://${host}:${port}`;
}

export interface SourceMeta {
  key: string;
  name: string;
  syncedAt: string | null;
}

export interface SearchResult {
  id: number;
  parliamentaryName: string;
  uf: string | null;
}

export interface SearchResponse {
  query: string;
  results: SearchResult[];
  sources: SourceMeta[];
}

export interface ProfilePolitician {
  id: number;
  civilName: string;
  parliamentaryName: string;
  uf: string | null;
  party: string | null;
}

export interface ExpenseItem {
  category: string;
  supplierName: string | null;
  issuedAt: string | null;
  amountNet: number;
  documentUrl: string | null;
}

export interface ExpensesLinked {
  linked: true;
  year: number;
  availableYears: number[];
  total: number;
  monthly: { month: number; total: number }[];
  largestItems: ExpenseItem[];
  source: SourceMeta;
}

export interface ExpensesUnlinked {
  linked: false;
  message: string;
  source: SourceMeta;
}

export interface ProfileResponse {
  politician: ProfilePolitician;
  summary: string;
  // Provenance for the identity header (registry sync); the expenses
  // section carries its own source block.
  sources: SourceMeta[];
  expenses: ExpensesLinked | ExpensesUnlinked;
}

async function getJSON<T>(path: string): Promise<T | null> {
  try {
    const res = await fetch(`${apiURL()}${path}`, { cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()) as T;
  } catch {
    return null;
  }
}

export function searchPoliticians(q: string): Promise<SearchResponse | null> {
  return getJSON<SearchResponse>(
    `/api/politicians/search?q=${encodeURIComponent(q)}`,
  );
}

export async function getProfile(
  id: number,
  year?: number,
): Promise<ProfileResponse | null | "not-found"> {
  try {
    const qs = year ? `?year=${year}` : "";
    const res = await fetch(`${apiURL()}/api/politicians/${id}${qs}`, {
      cache: "no-store",
    });
    if (res.status === 404) return "not-found";
    if (!res.ok) return null;
    return (await res.json()) as ProfileResponse;
  } catch {
    return null;
  }
}
