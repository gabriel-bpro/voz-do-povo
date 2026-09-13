// Per-dataset provenance line, locked by the prototype's `.fonte` pattern:
// "Fonte: {source name} · última sincronização: {date}".

import type { SourceMeta } from "@/lib/api";
import { formatSyncDate } from "@/lib/format";

export function SourceLine({ source }: { source: SourceMeta }) {
  return (
    <span className="fonte">
      Fonte: {source.name} · última sincronização:{" "}
      {formatSyncDate(source.syncedAt)}
    </span>
  );
}
