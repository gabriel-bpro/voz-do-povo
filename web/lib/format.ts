// pt-BR display helpers: money, dates, and the human-scale anchors locked by
// the report prototype ("≈ 268 salários mínimos").

export const brl = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
  maximumFractionDigits: 0,
});

export const brlCents = new Intl.NumberFormat("pt-BR", {
  style: "currency",
  currency: "BRL",
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
});

// Salário mínimo por ano (decretos federais) — usado para a âncora humana.
const SALARIO_MINIMO: Record<number, number> = {
  2023: 1320,
  2024: 1412,
  2025: 1518,
  2026: 1621, // Decreto 12.797/2025
};

// salariosMinimosAnchor: "≈ 268 salários mínimos" — null when the year has
// no reference wage on file.
export function salariosMinimosAnchor(total: number, year: number): string | null {
  const sm = SALARIO_MINIMO[year];
  if (!sm || total <= 0) return null;
  return `≈ ${Math.round(total / sm).toLocaleString("pt-BR")} salários mínimos`;
}

export function formatSyncDate(iso: string | null): string {
  if (!iso) return "nunca sincronizado";
  const d = new Date(iso);
  return d.toLocaleDateString("pt-BR", { timeZone: "UTC" });
}

export const MONTHS_SHORT = [
  "jan", "fev", "mar", "abr", "mai", "jun",
  "jul", "ago", "set", "out", "nov", "dez",
];
