import { defineRailway, postgres, preserve, project, service, volume } from "railway/iac";

// Voz do Povo — MVP topology (spec: .scratch/voz-do-povo-mvp/spec.md).
// 4 units: Next web, Go api, Go worker, Postgres. Private networking only
// between services. Deploys happen via `railway up` (no GitHub remote yet):
// api/worker build from the repo-root Dockerfile (each picks its binary via
// `start`); web builds from web/Dockerfile with web/ as the upload context.

export default defineRailway(() => {
  const Postgres = postgres("Postgres", { region: "iad" });
  Postgres.networking = { privateNetworkEndpoint: "postgres" };
  const postgresVolume = volume("postgres-volume", { alerts: { usage: { "100": {}, "80": {}, "95": {} } }, allowOnlineResize: true, region: "iad", sizeMB: 4096 });

  const api = service("api", {
    start: "/bin/api",
    healthcheck: "/api/healthz",
    replicas: { "iad": 1 },
    env: {
      PORT: "8080",
      DATABASE_URL: Postgres.env.DATABASE_URL,
    },
  });

  // Cron service: Railway starts the container daily at 06:00 UTC
  // (03:00 BRT), the worker runs every sync once and exits. No restart —
  // a failed run surfaces as a failed cron execution, retried next day.
  const worker = service("worker", {
    start: "/bin/worker",
    deploy: {
      cronSchedule: "0 6 * * *",
      restartPolicyType: "NEVER",
      numReplicas: 1,
    },
    env: {
      DATABASE_URL: Postgres.env.DATABASE_URL,
      GCP_SERVICE_ACCOUNT_JSON: preserve(),
    },
  });

  const web = service("web", {
    replicas: { "iad": 1 },
    env: {
      API_HOST: api.env.RAILWAY_PRIVATE_DOMAIN,
      API_PORT: "8080",
    },
  });

  return project("voz-do-povo", {
    resources: [worker, api, Postgres, web, postgresVolume],
  });
});
