// Politician profile (issue 13): templated identity header (zero LLM) + the
// expenses section in the locked prototype style — stat card, big number,
// monthly bars, largest items, per-dataset source line with sync date, and
// the honest "not linked" state when identity can't be confidently linked.

import Link from "next/link";
import { notFound } from "next/navigation";
import { getProfile, type ProfileResponse } from "@/lib/api";
import { SourceLine } from "@/components/SourceLine";
import {
  brl,
  brlCents,
  salariosMinimosAnchor,
  MONTHS_SHORT,
} from "@/lib/format";

export default async function ProfilePage({
  params,
  searchParams,
}: PageProps<"/politicos/[id]">) {
  const { id } = await params;
  const { year: yearParam } = await searchParams;
  const politicianID = Number(id);
  if (!Number.isInteger(politicianID) || politicianID <= 0) notFound();

  const year = yearParam ? Number(yearParam) : undefined;
  const profile = await getProfile(
    politicianID,
    year && Number.isInteger(year) ? year : undefined,
  );
  if (profile === "not-found") notFound();

  return (
    <main className="wrap">
      {profile ? (
        <Profile profile={profile} />
      ) : (
        <p className="empty-note">API indisponível no momento.</p>
      )}
    </main>
  );
}

function Profile({ profile }: { profile: ProfileResponse }) {
  const { politician: p, summary, sources, expenses } = profile;
  // Data-only sub-line: the templated sentence (cargo, mandato) is the API's
  // summary — one template, one owner.
  const subtitle = p.party && p.uf ? `${p.party}–${p.uf}` : (p.party ?? p.uf);

  return (
    <>
      <header className="card">
        {sources[0] && (
          <div style={{ display: "flex", justifyContent: "flex-end" }}>
            <SourceLine source={sources[0]} />
          </div>
        )}
        <h1 style={{ fontSize: 28 }}>{p.parliamentaryName}</h1>
        {subtitle && (
          <p className="sub-line" style={{ fontSize: 14 }}>
            {subtitle}
          </p>
        )}
        <p style={{ marginTop: 14 }}>{summary}</p>
        {expenses.linked && expenses.flaggedCount > 0 && (
          <p className="rollup-attention">
            <a href="#gastos">
              {expenses.flaggedCount === 1
                ? "1 despesa chama atenção"
                : `${expenses.flaggedCount} despesas chamam atenção`}
            </a>
          </p>
        )}
      </header>

      <section className="card" id="gastos">
        <div className="card-header">
          <h2>
            Gastos {expenses.linked ? expenses.year : ""} ·{" "}
            <span
              className="tooltip"
              title="Cota parlamentar: verba que cada deputado(a) recebe para custear o trabalho do mandato — passagens, escritório, divulgação."
            >
              cota parlamentar
            </span>
          </h2>
          <SourceLine source={expenses.source} />
        </div>

        {!expenses.linked ? (
          <div className="not-linked">
            <strong>Dados não ligados.</strong>
            <p>{expenses.message}</p>
          </div>
        ) : (
          <Expenses expenses={expenses} politicianID={p.id} />
        )}
      </section>
    </>
  );
}

function Expenses({
  expenses,
  politicianID,
}: {
  expenses: Extract<ProfileResponse["expenses"], { linked: true }>;
  politicianID: number;
}) {
  const anchor = salariosMinimosAnchor(expenses.total, expenses.year);
  const maxMonth = Math.max(...expenses.monthly.map((m) => m.total), 0);

  return (
    <>
      <div className="big-number">{brl.format(expenses.total)}</div>
      {anchor && (
        <p className="sub-line">
          {anchor} · soma dos gastos líquidos de {expenses.year}
        </p>
      )}

      <div className="explainer">
        ℹ️ A cota parlamentar é o dinheiro que cada deputado(a) recebe para
        custear o mandato: passagens, escritório, divulgação. Valores aqui são
        os líquidos — já descontadas as glosas (valores devolvidos ou
        desautorizados).
      </div>

      {expenses.availableYears.length > 0 && (
        <nav className="pills" aria-label="Selecionar ano">
          {expenses.availableYears.map((y) => (
            <Link
              key={y}
              href={`/politicos/${politicianID}?year=${y}`}
              className={y === expenses.year ? "pill active" : "pill"}
            >
              {y}
            </Link>
          ))}
        </nav>
      )}

      <div className="trend" role="img" aria-label="Gastos mês a mês">
        {expenses.monthly.map((m) => (
          <div
            key={m.month}
            className="trend-bar"
            style={{
              height: `${maxMonth > 0 ? Math.max((m.total / maxMonth) * 100, 3) : 3}%`,
            }}
            title={`${MONTHS_SHORT[m.month - 1]}: ${brl.format(m.total)}`}
          />
        ))}
      </div>
      <div className="trend-labels">
        {expenses.monthly.map((m) => (
          <span key={m.month}>{MONTHS_SHORT[m.month - 1]}</span>
        ))}
      </div>

      <table className="data">
        <thead>
          <tr>
            <th>Maiores gastos</th>
            <th style={{ textAlign: "right" }}>Valor</th>
          </tr>
        </thead>
        <tbody>
          {expenses.largestItems.map((it, i) => (
            <tr key={i}>
              <td>
                {it.category}
                <div className="item-sub">
                  {[it.supplierName, it.issuedAt].filter(Boolean).join(" · ")}
                </div>
                {it.badge && (
                  <div className="badge-attention">
                    Chama atenção · Acima da mediana da categoria &quot;
                    {it.category}&quot; (mediana: {brlCents.format(it.badge.median)})
                  </div>
                )}
              </td>
              <td className="num">{brlCents.format(it.amountNet)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
