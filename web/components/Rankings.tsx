// Home rankings block (issue 16): the two expense rankings — top spenders
// of the current calendar year and biggest single expenses of the latest
// synced month — under one Deputados/Senadores toggle. Locked prototype
// style: compact tables with bars, source + sync-date line, no editorial
// content, no badges on ranking rows. The toggle is plain links (server
// component, zero client JS) and preserves an active search query.

import Link from "next/link";
import type { House, RankingsResponse, RankingPerson } from "@/lib/api";
import { brl, brlCents, MONTHS_LONG } from "@/lib/format";
import { SourceLine } from "@/components/SourceLine";

function houseHref(house: House, q: string): string {
  const params = new URLSearchParams();
  if (house === "senado") params.set("casa", "senado");
  if (q) params.set("q", q);
  const qs = params.toString();
  return qs ? `/?${qs}` : "/";
}

function personLabel(p: RankingPerson): string {
  const tag = p.party && p.uf ? ` (${p.party}–${p.uf})` : p.uf ? ` (${p.uf})` : "";
  return p.parliamentaryName + tag;
}

export function Rankings({
  rankings,
  house,
  query,
}: {
  rankings: RankingsResponse;
  house: House;
  query: string;
}) {
  const monthName = rankings.month > 0 ? MONTHS_LONG[rankings.month - 1] : "";

  return (
    <section aria-label="Rankings">
      <nav className="pills" aria-label="Alternar casa legislativa">
        <Link
          href={houseHref("camara", query)}
          className={house === "camara" ? "pill active" : "pill"}
        >
          Deputados
        </Link>
        <Link
          href={houseHref("senado", query)}
          className={house === "senado" ? "pill active" : "pill"}
        >
          Senadores
        </Link>
      </nav>

      <div className="ranking-grid">
        <section className="card">
          <div className="card-header">
            <h2>Maiores gastos da cota em {rankings.year}</h2>
            <SourceLine source={rankings.source} />
          </div>
          <RankingTable
            rows={rankings.topSpenders.map((r) => ({
              person: r.politician,
              sub: null,
              value: r.total,
            }))}
            cents={false}
            emptyNote="Nenhum gasto sincronizado ainda neste ano."
          />
        </section>

        <section className="card">
          <div className="card-header">
            <h2>
              Maiores gastos únicos de {monthName} de {rankings.monthYear}
            </h2>
            <SourceLine source={rankings.source} />
          </div>
          <RankingTable
            rows={rankings.biggestExpenses.map((r) => ({
              person: r.politician,
              sub: [r.category, r.supplierName, r.issuedAt]
                .filter(Boolean)
                .join(" · "),
              value: r.amountNet,
            }))}
            cents
            emptyNote="Nenhum gasto sincronizado ainda neste mês."
          />
        </section>
      </div>
    </section>
  );
}

// Compact table with bars (prototype's barRows): rows arrive sorted desc,
// so the first row's value is the bar scale.
function RankingTable({
  rows,
  cents,
  emptyNote,
}: {
  rows: { person: RankingPerson; sub: string | null; value: number }[];
  cents: boolean;
  emptyNote: string;
}) {
  if (rows.length === 0) {
    return <p className="empty-note">{emptyNote}</p>;
  }
  const max = Math.max(...rows.map((r) => r.value), 1);
  const money = cents ? brlCents : brl;
  return (
    <table className="data">
      <tbody>
        {rows.map((r, i) => (
          <tr key={r.person.id + "-" + i}>
            <td>
              <Link href={`/politicos/${r.person.id}`}>
                {personLabel(r.person)}
              </Link>
              {r.sub && <div className="item-sub">{r.sub}</div>}
            </td>
            <td className="num">{money.format(r.value)}</td>
            <td className="bar-cell">
              <div
                className="bar"
                style={{ width: `${Math.max((r.value / max) * 100, 2)}%` }}
              />
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
