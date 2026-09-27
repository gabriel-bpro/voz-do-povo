// Home: the citizen's entry point — the politician search bar front and
// center, the one-line project explainer (issue 13), and the expense
// rankings block under a Deputados/Senadores toggle (issue 16).

import Link from "next/link";
import {
  getRankings,
  searchPoliticians,
  type House,
} from "@/lib/api";
import { SourceLine } from "@/components/SourceLine";
import { Rankings } from "@/components/Rankings";

export default async function Home({
  searchParams,
}: {
  searchParams: Promise<{ q?: string; casa?: string }>;
}) {
  const { q, casa } = await searchParams;
  const query = (q ?? "").trim();
  const house: House = casa === "senado" ? "senado" : "camara";

  const [search, rankings] = await Promise.all([
    query ? searchPoliticians(query) : Promise.resolve(null),
    getRankings(house),
  ]);

  return (
    <main className="wrap">
      <header className="hero">
        <h1>
          Voz do <span className="accent">Povo</span>
        </h1>
        <p className="explainer-line">
          Dados públicos sobre os políticos que você elegeu, explicados em
          linguagem simples.
        </p>
        <form className="search-form" action="/" method="get">
          {casa === "senado" && <input type="hidden" name="casa" value="senado" />}
          <input
            type="search"
            name="q"
            defaultValue={query}
            placeholder="Buscar parlamentar por nome…"
            aria-label="Buscar parlamentar por nome"
            autoFocus
          />
        </form>
      </header>

      {search && (
        <section className="card">
          <div className="card-header">
            <h2>Resultados</h2>
            {search.sources[0] && <SourceLine source={search.sources[0]} />}
          </div>
          {search.results.length === 0 ? (
            <p className="empty-note">
              Nenhum parlamentar encontrado para “{search.query}”.
            </p>
          ) : (
            search.results.map((r) => (
              <Link
                key={r.id}
                href={`/politicos/${r.id}`}
                className="result-row"
              >
                <span>{r.parliamentaryName}</span>
                {r.uf && <span className="result-uf">{r.uf}</span>}
              </Link>
            ))
          )}
        </section>
      )}

      {query && !search && (
        <p className="empty-note">API indisponível no momento.</p>
      )}

      {rankings && <Rankings rankings={rankings} house={house} query={query} />}
    </main>
  );
}
