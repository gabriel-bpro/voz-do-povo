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
  // Attention badge (rule 1): present only when the item sits above its
  // bucket's trailing-12-month median; carries the bare median, never a
  // multiplier.
  badge?: { median: number };
}

export interface ExpensesLinked {
  linked: true;
  year: number;
  availableYears: number[];
  total: number;
  monthly: { month: number; total: number }[];
  largestItems: ExpenseItem[];
  // Header rollup: how many of the year's expenses carry the badge.
  flaggedCount: number;
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
  emendas: EmendasLinked | EmendasUnlinked;
}

// --- profile emendas section (issue 17) ---

export interface EmendaYearTotal {
  year: number;
  empenhado: number;
  pago: number;
}

export interface EmendaBreakdownRow {
  label: string;
  total: number;
}

export interface EmendaTop {
  code: string;
  year: number;
  tipo: string;
  // Permanent type marker (transferência especial), NOT an attention badge.
  isPix: boolean;
  localidade: string | null;
  funcao: string | null;
  empenhado: number;
  pago: number;
}

export interface EmendasLinked {
  linked: true;
  year: number;
  years: EmendaYearTotal[];
  byArea: EmendaBreakdownRow[];
  byLocalidade: EmendaBreakdownRow[];
  top: EmendaTop[];
  // Attention badge (rule 3): present only when >= 50% of the current
  // calendar year's empenhado is emenda PIX; carries the actual percentage
  // and the year it was computed on (always the current year, never ?year=).
  badge: { pixPercent: number; year: number } | null;
  source: SourceMeta;
}

export interface EmendasUnlinked {
  linked: false;
  message: string;
  source: SourceMeta;
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

// --- home expense rankings (issue 16) ---

export type House = "camara" | "senado";

export interface RankingPerson {
  id: number;
  parliamentaryName: string;
  uf: string | null;
  party: string | null;
}

export interface TopSpender {
  politician: RankingPerson;
  total: number;
}

export interface BiggestExpense {
  politician: RankingPerson;
  category: string;
  supplierName: string | null;
  issuedAt: string | null;
  amountNet: number;
}

export interface RankingsResponse {
  house: House;
  // Year of the top-spenders ranking (current calendar year).
  year: number;
  // Month of the biggest-single-expenses ranking: the latest month the sync
  // has landed for the house (graceful degradation, not "now").
  month: number;
  monthYear: number;
  topSpenders: TopSpender[];
  biggestExpenses: BiggestExpense[];
  source: SourceMeta;
}

export function getRankings(house: House): Promise<RankingsResponse | null> {
  return getJSON<RankingsResponse>(`/api/rankings?house=${house}`);
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
