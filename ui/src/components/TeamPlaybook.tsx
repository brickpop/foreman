import type { TeamConfig } from "../api/types/home";
import { resolveWho } from "../app/model";

// A team's playbook and escalation rules, as the Team page and the Chief of
// Staff's preview show them (wiki/product/models/team-definition.md#playbook-and-escalation).

export function TeamPlaybook({ team }: { team: TeamConfig }) {
  const who = (w: string) => {
    const r = resolveWho(team, w);
    return r.known ? (
      r.text
    ) : (
      <span className="flex items-center gap-1" title="No member matches this; check the team file">
        {r.text} <span className="text-acc">⚠</span>
      </span>
    );
  };
  return (
    <>
      <div className="grid grid-cols-[28px_110px_1fr] gap-x-3 gap-y-2.5">
        {(team.playbook ?? []).flatMap((step, i) =>
          Object.entries(step).map(([w, text]) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: a step has no id; its place in the playbook is its identity
            <div key={`${i}-${w}`} className="contents">
              <span className="font-mono text-tx4">{i + 1}</span>
              <span className="font-semibold">{who(w)}</span>
              <span className="text-tx2">{text}</span>
            </div>
          )),
        )}
      </div>
      {(team.escalate ?? []).length > 0 && (
        <div className="flex flex-col gap-2">
          <span className="text-[13px] font-medium text-tx4">Escalate</span>
          {team.escalate.map((e) => (
            <span key={e.when} className="flex gap-2">
              <span className="text-tx2">{e.when}</span>
              <span>→</span>
              <span className={`font-medium ${e.to === "ceo" ? "text-accTx" : ""}`}>
                {who(e.to)}
              </span>
            </span>
          ))}
        </div>
      )}
    </>
  );
}
