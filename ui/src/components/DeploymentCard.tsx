import { Link, useNavigate } from "@tanstack/react-router";
import { roleLabel, type SessionGroup, when } from "../app/model";
import { Avatar } from "./Avatar";
import { card, cover, Dot, LeadTag } from "./ui";

/** One active deployment: its team, goal and awake agents, each opening its
 * session; the asleep ones only counted. The card opens the deployment's page. */
export function DeploymentCard({ group: g }: { group: SessionGroup }) {
  const navigate = useNavigate();
  const working = g.agents.filter((a) => a.instance.status === "running").length;
  return (
    <article className={`${card} !gap-2.5 !px-0 !pb-2 hover:border-line3`}>
      <header className="flex flex-col gap-0.5 px-4">
        <span className="flex items-baseline gap-2">
          <Link
            to="/sessions"
            search={{ deployment: g.deployment }}
            className={`font-semibold ${cover}`}
          >
            {g.team?.config.name || g.deployment.split("@")[0]}
          </Link>
          <span className="text-[13px] text-tx4">on {g.project}</span>
          {g.needsYou && <Dot title="Needs you" />}
          <span className="flex-1" />
          <span className={`text-[12.5px] ${working > 0 ? "text-ok" : "text-tx4"}`}>
            {working > 0 ? `${working} working` : g.agents.length > 0 ? "idle" : "asleep"}
          </span>
        </span>
        {g.goal && <p className="line-clamp-2 text-[13.5px] text-tx3">{g.goal}</p>}
      </header>
      <div className="flex flex-col">
        {g.agents.map(({ instance: i, agent, lead }) => (
          <button
            key={i.id}
            type="button"
            onClick={() =>
              navigate({ to: "/sessions", search: { deployment: g.deployment, agent: i.id } })
            }
            className="relative z-10 grid grid-cols-[auto_1fr] items-center gap-x-2.5 px-4 py-1.5 text-left hover:bg-hover"
          >
            <Avatar look={agent?.soul.look} size={28} status={i.status} dotBorder="var(--panel)" />
            <span className="flex min-w-0 flex-col">
              <span className="flex items-center gap-1.5 text-[13.5px]">
                <span className="font-medium">{agent?.soul.name ?? i.agentId}</span>
                {lead && <LeadTag />}
                <span className="truncate text-tx4">{agent ? roleLabel(agent.soul.role) : ""}</span>
              </span>
              <span className="truncate font-mono text-[11.5px] text-tx4">
                {i.doing ? `${i.doing.line} · ${when(i.doing.at)}` : i.status}
              </span>
            </span>
          </button>
        ))}
        {g.asleep.length > 0 && (
          <span className="flex items-center gap-2.5 px-4 py-1.5 text-[12.5px] text-tx4">
            <span className="flex">
              {g.asleep.slice(0, 5).map(({ instance: i, agent }, n) => (
                <span
                  key={i.id}
                  className={n > 0 ? "-ml-1.5" : ""}
                  title={agent?.soul.name ?? i.agentId}
                >
                  <Avatar look={agent?.soul.look} size={20} />
                </span>
              ))}
            </span>
            {g.asleep.length} asleep · woken when needed
          </span>
        )}
      </div>
    </article>
  );
}
