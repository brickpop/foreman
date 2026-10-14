import { useParams } from "@tanstack/react-router";
import { DemoBadge } from "../components/DemoBadge";
import { PageTitle } from "../components/ui";
import { ComingWith } from "./Placeholder";

export type TeamTab = "deployments" | "members" | "playbook" | "file";

// wiki/product/features/team-page.md
export function Team() {
  const { team } = useParams({ from: "/teams/$team" });
  return (
    <div className="mx-auto flex max-w-[920px] flex-col gap-8 px-12 pt-10 pb-16">
      <PageTitle title={team} subtitle={<DemoBadge areas={["workspace", "agents"]} />} />
      <ComingWith card="Team and Agent pages, and the Deploy dialog" />
    </div>
  );
}
