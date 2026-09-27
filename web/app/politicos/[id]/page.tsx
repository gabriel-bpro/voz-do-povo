// Politician profile (issue 13): templated identity header (zero LLM) + the
// expenses section in the locked prototype style — stat card, big number,
// monthly bars, largest items, per-dataset source line with sync date, and
// the honest "not linked" state when identity can't be confidently linked.

import Link from "next/link";
import { notFound } from "next/navigation";
import {
  getProfile,
  type ProfileResponse,
  type EmendasLinked,
} from "@/lib/api";
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
  const { politician: p, summary, sources, expenses, emendas } = profile;
  // Data-only sub-line: the templated sentence (cargo, mandato) is the API's
  // summary — one template, one owner.
  const subtitle = p.party && p.uf ? `${p.party}–${p.uf}` : (p.party ?? p.uf);
  // Header rollup: flagged expenses (rule 1) + the emendas PIX badge (rule 3).
  const attentionCount =
    (expenses.linked ? expenses.flaggedCount : 0) +
    (emendas.linked && emendas.badge ? 1 : 0);
  const attentionAnchor =
    expenses.linked && expenses.flaggedCount > 0 ? "#gastos" : "#emendas";

  return (
    <>
      <header className="card">
        {sources[0] && (
          <div className="profile-source">
            <SourceLine source={sources[0]} />
          </div>
        )}
        <h1 className="profile-title">{p.parliamentaryName}</h1>
        {subtitle && <p className="profile-subtitle">{subtitle}</p>}
        <p className="profile-summary">{summary}</p>
        {attentionCount > 0 && (
          <p className="rollup-attention">
            <a href={attentionAnchor}>
              {attentionCount === 1
                ? "1 item chama atenção"
                : `${attentionCount} itens chamam atenção`}
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
              title="Cota parlamentar: verba que cada parlamentar recebe para custear o trabalho do mandato — passagens, escritório, divulgação."
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

      <section className="card" id="emendas">
        <div className="card-header">
          <h2>
            Emendas parlamentares {emendas.linked ? emendas.year : ""} ·{" "}
            <span
              className="tooltip"
              title="Emenda PIX (transferência especial) vai direto para o fundo da prefeitura ou do estado, sem objeto específico. É o tipo de emenda mais difícil de rastrear."
            >
              emenda PIX
            </span>
          </h2>
          <SourceLine source={emendas.source} />
        </div>

        {!emendas.linked ? (
          <div className="not-linked">
            <strong>Dados não ligados.</strong>
            <p>{emendas.message}</p>
          </div>
        ) : (
          <Emendas emendas={emendas} />
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
        A cota parlamentar é o dinheiro que cada parlamentar recebe para
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

// Emendas section (issue 17), locked prototype style: stat trio (empenhado /
// pago / % pago) for the selected year, yearly totals table, breakdowns by
// policy area and by localidade with bars, top 5 with the permanent PIX type
// marker, and the plain-language explainer.
function Emendas({ emendas }: { emendas: EmendasLinked }) {
  const current = emendas.years.find((y) => y.year === emendas.year);
  const empenhado = current?.empenhado ?? 0;
  const pago = current?.pago ?? 0;
  const pctPago = empenhado > 0 ? Math.round((pago / empenhado) * 100) : 0;
  const anchor = salariosMinimosAnchor(empenhado, emendas.year);

  return (
    <>
      {emendas.badge && (
        <div className="badge-attention">
          Chama atenção · {emendas.badge.pixPercent}% das emendas empenhadas
          em {emendas.badge.year} são emendas PIX
        </div>
      )}
      <div className="stat-trio">
        <div>
          <div className="fonte">Empenhado (reservado)</div>
          <div className="stat-value">{brl.format(empenhado)}</div>
        </div>
        <div>
          <div className="fonte">Pago de fato</div>
          <div className="stat-value">{brl.format(pago)}</div>
        </div>
        <div>
          <div className="fonte">% pago</div>
          <div className="stat-value">{pctPago}%</div>
        </div>
      </div>
      {anchor && <p className="sub-line">{anchor} empenhados em {emendas.year}</p>}

      <div className="explainer">
        Emenda parlamentar é dinheiro do orçamento federal que o parlamentar
        direciona para uma cidade, estado ou entidade. Empenhado é o valor
        reservado; pago é o que efetivamente saiu do cofre. A diferença
        importa: emenda empenhada e não paga pode indicar obra parada ou
        destino travado.
      </div>

      <table className="data">
        <thead>
          <tr>
            <th>Ano</th>
            <th style={{ textAlign: "right" }}>Empenhado</th>
            <th style={{ textAlign: "right" }}>Pago</th>
            <th style={{ textAlign: "right" }}>% pago</th>
          </tr>
        </thead>
        <tbody>
          {emendas.years.map((y) => (
            <tr key={y.year}>
              <td>{y.year}</td>
              <td className="num">{brl.format(y.empenhado)}</td>
              <td className="num">{brl.format(y.pago)}</td>
              <td className="num">
                {y.empenhado > 0 ? Math.round((y.pago / y.empenhado) * 100) : 0}%
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {emendas.byArea.length > 0 && (
        <BreakdownTable title={`Por área (${emendas.year})`} rows={emendas.byArea} />
      )}

      {emendas.byLocalidade.length > 0 && (
        <BreakdownTable
          title={`Por estado/município (${emendas.year})`}
          rows={emendas.byLocalidade}
        />
      )}

      {emendas.top.length > 0 && (
        <table className="data">
          <thead>
            <tr>
              <th>Maiores emendas de {emendas.year}</th>
              <th style={{ textAlign: "right" }}>Empenhado</th>
            </tr>
          </thead>
          <tbody>
            {emendas.top.map((e) => (
              <tr key={e.code}>
                <td>
                  {e.localidade ?? "Localidade não informada"}{" "}
                  {e.isPix && <span className="badge-pix">PIX</span>}
                  <div className="item-sub">
                    {[e.funcao, e.tipo].filter(Boolean).join(" · ")}
                  </div>
                </td>
                <td className="num">{brl.format(e.empenhado)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <p className="fonte" style={{ marginTop: 10 }}>
        Emenda = dinheiro federal direcionado pelo parlamentar.{" "}
        <strong>PIX</strong> = sem objeto específico, a mais difícil de
        rastrear.
      </p>
    </>
  );
}

// Emendas breakdown (policy area / localidade): label, proportional bar,
// empenhado — one shape, both groupings.
function BreakdownTable({
  title,
  rows,
}: {
  title: string;
  rows: { label: string; total: number }[];
}) {
  const max = Math.max(...rows.map((r) => r.total), 0);
  return (
    <table className="data">
      <thead>
        <tr>
          <th>{title}</th>
          <th />
          <th style={{ textAlign: "right" }}>Empenhado</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((r) => (
          <tr key={r.label}>
            <td>{r.label}</td>
            <td className="bar-cell">
              <div
                className="bar"
                style={{
                  width: `${max > 0 ? Math.max((r.total / max) * 100, 2) : 2}%`,
                }}
              />
            </td>
            <td className="num">{brl.format(r.total)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
