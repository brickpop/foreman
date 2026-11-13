import { Link } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import { readable, type TeamCard, teamCards } from "../app/model";
import { useInstances, useProjects, useTeams } from "../app/queries";
import { DeployDialog } from "./DeployDialog";
import { Button, Card, cover, Loading, StatusPill } from "./ui";

// The teams as cards, each opening its page, then a card to create one: on the
// Dashboard and in Settings → Teams, the same (wiki/product/features/dashboard.md).
export function TeamGrid() {
  const teams = useTeams();
  const instances = useInstances();
  const projects = useProjects();
  const openChiefOfStaff = useOpenChiefOfStaff();
  const [deploy, setDeploy] = useState<string>();
  const cards = teamCards(teams.data ?? [], instances.data ?? []);
  const canDeploy = (projects.data ?? []).length > 0;
  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(240px,1fr))] gap-3">
      {teams.isPending && <Loading className="min-h-[132px]" />}
      {cards.map((t) => (
        <TeamCardView
          key={t.id}
          team={t}
          onDeploy={canDeploy ? () => setDeploy(t.id) : undefined}
        />
      ))}
      <AddCard onClick={() => openChiefOfStaff({ start: "create-team" })}>
        <span className="font-semibold text-tx2">+ New team</span>
        <span>The Chief of Staff drafts it with you.</span>
      </AddCard>
      {deploy && <DeployDialog open onClose={() => setDeploy(undefined)} team={deploy} />}
    </div>
  );
}

function TeamCardView({ team: t, onDeploy }: { team: TeamCard; onDeploy?: () => void }) {
  return (
    <Card
      opens
      head={
        <>
          <Link
            to="/settings/teams/$team"
            params={{ team: t.id }}
            className={`min-w-0 flex-1 truncate text-[15px] font-semibold ${cover}`}
          >
            {t.name}
          </Link>
          {t.on.length > 0 ? (
            <StatusPill color="var(--ok)">deployed</StatusPill>
          ) : (
            <StatusPill color="var(--tx4)">not deployed</StatusPill>
          )}
        </>
      }
    >
      <p className="line-clamp-2 min-h-[2lh] text-[13.5px] text-tx3">
        {t.purpose || "No purpose written yet."}
      </p>
      <span
        className="truncate text-[12.5px] text-tx4"
        title={t.members.map((s) => s.name).join(", ")}
      >
        {t.members.length} {t.members.length === 1 ? "agent" : "agents"}
        {t.members.length > 0 && `: ${t.members.map((s) => s.name).join(", ")}`}
      </span>
      <span className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-[12.5px] text-tx4">
          {t.on.length > 0 && `On ${t.on.map(readable).join(", ")}`}
        </span>
        {onDeploy && (
          <Button variant="ghost" className="relative z-10" onClick={onDeploy}>
            Deploy
          </Button>
        )}
      </span>
    </Card>
  );
}

/** A dashed card that adds one more (a project, a team). */
export function AddCard({ onClick, children }: { onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex min-h-[132px] flex-col items-start justify-center gap-1 rounded-xl border border-dashed border-line2 px-4 py-3.5 text-left text-[13px] text-tx4 hover:border-line3 hover:text-tx3"
    >
      {children}
    </button>
  );
}
