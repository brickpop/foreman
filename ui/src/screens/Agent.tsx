import { useParams } from "@tanstack/react-router";
import { DemoBadge } from "../components/DemoBadge";
import { PageTitle } from "../components/ui";
import { ComingWith } from "./Placeholder";

// wiki/product/features/agent-page.md
export function Agent() {
  const { team, agent } = useParams({ from: "/teams/$team/agents/$agent" });
  return (
    <div className="mx-auto flex max-w-[920px] flex-col gap-8 px-12 pt-10 pb-16">
      <PageTitle
        title={agent}
        subtitle={
          <>
            on team {team} <DemoBadge areas={["workspace"]} />
          </>
        }
      />
      <ComingWith card="Team and Agent pages, and the Deploy dialog" />
    </div>
  );
}
