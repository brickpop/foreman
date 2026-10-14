import { type ForemanClient, useClient } from "../api/client";

// "demo data": shown wherever what is on screen comes from the demo client,
// so demo data is never mistaken for real (the Full UI demo epic's condition).

export function DemoBadge({ areas }: { areas: (keyof ForemanClient)[] }) {
  const client = useClient();
  if (!areas.some((a) => client[a].demo)) return null;
  return (
    <span
      title="Served by the demo client: not real data yet"
      className="rounded-[5px] border border-dashed border-team px-1.5 py-px font-mono text-[10.5px] font-semibold uppercase tracking-[.05em] text-team"
    >
      demo data
    </span>
  );
}
