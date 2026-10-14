# voz-do-povo

Um app web para ajudar cidadãos a entender, investigar e fazer due diligence dos políticos que elegeram — construído sobre dados abertos do governo brasileiro, explicados em linguagem simples (produto em pt-BR, código em inglês).

O app mantém seu próprio **espelho** dos dados abertos do governo, sincronizado conforme a cadência de cada fonte; nunca chama APIs do governo em tempo de requisição. Os dados chegam em uma **camada raw** (staging, verbatim, com proveniência) e são promovidos para uma **camada curada** — o único schema que o app consulta.

## Estrutura do repositório

| Caminho | Descrição |
|---|---|
| `cmd/api` | Serviço de API — serve a camada curada via HTTP |
| `cmd/worker` | Serviço worker — executa as sincronizações de dados |
| `internal/store` | Acesso ao Postgres + migrations embutidas (`internal/store/migrations/`) |
| `internal/httpapi` | Handlers HTTP |
| `internal/sync` | Desembarque no raw + promoção para o curado, linkage de identidade |
| `internal/camara` | Adaptador da fonte Câmara dos Deputados (só fetch + parse, sem DB) |
| `internal/testharness` | Fixtures de teste e helpers de Postgres descartável |
| `web/` | App Next.js (App Router) — veja `web/AGENTS.md` |

## Pré-requisitos

- Go 1.25+
- Node.js + npm
- Docker (obrigatório para os testes — ambas as frentes de teste sobem um Postgres descartável)

## Desenvolvimento

```bash
# Serviços Go
go build ./...
go vet ./...
go test ./...        # requer o daemon do Docker rodando

# Web
cd web && npm install
npm run lint
npm run build
```

Configuração via `.env` — veja `.env.example`.

## Testes

Duas frentes acordadas:

1. **Frente da API HTTP** — exercita a API via HTTP contra um Postgres real semeado na camada curada (`testharness.API`).
2. **Frente de sync** — alimenta fixtures de fonte gravadas nos pontos de entrada do sync, e verifica o estado das tabelas raw + curadas (`testharness.TestDB` + `testharness.Fixture`).

Ambas sobem um Postgres descartável via Docker. O frontend não tem camada de testes (só type-check + lint).

## Rodando localmente

```bash
docker compose up
```

Sobe Postgres, a API, o worker e o app web. Configuração via `.env` — veja `.env.example`.

## Deploy

Independente de provedor: o `Dockerfile` da raiz gera os dois binários Go (`/bin/api`, `/bin/worker`) e `web/Dockerfile` gera o servidor standalone do Next.js. Faça deploy dos três serviços mais uma instância Postgres em qualquer plataforma de containers, configurando as variáveis de ambiente do `.env.example`.

## Documentação de domínio

- `CONTEXT.md` — glossário e linguagem do domínio
- `AGENTS.md` — guia do projeto orientado a agentes
