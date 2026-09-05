// Walking-skeleton home: proves the DB → API → web thread by rendering the
// database's own clock, served by the Go API. Real surfaces arrive with the
// later tickets (13+).

function apiURL(): string {
  const host = process.env.API_HOST ?? "localhost";
  const port = process.env.API_PORT ?? "8080";
  return `http://${host}:${port}`;
}

interface Health {
  database: string;
  serverTime: string;
}

async function getHealth(): Promise<Health | null> {
  try {
    const res = await fetch(`${apiURL()}/api/health`, { cache: "no-store" });
    if (!res.ok) return null;
    return (await res.json()) as Health;
  } catch {
    return null;
  }
}

export default async function Home() {
  const health = await getHealth();

  return (
    <main>
      <h1>Voz do Povo</h1>
      <p>
        Dados públicos sobre os políticos que você elegeu, explicados em
        linguagem simples.
      </p>
      {health ? (
        <p>
          Banco de dados conectado — horário do servidor:{" "}
          <time dateTime={health.serverTime}>
            {new Date(health.serverTime).toLocaleString("pt-BR", {
              timeZone: "UTC",
            })}
          </time>
        </p>
      ) : (
        <p>API indisponível no momento.</p>
      )}
    </main>
  );
}
