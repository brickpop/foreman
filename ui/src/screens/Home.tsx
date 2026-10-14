import { DemoBadge } from "../components/DemoBadge";
import { PageTitle } from "../components/ui";
import { ComingWith } from "./Placeholder";

// wiki/product/features/home.md
export function Home() {
  return (
    <div className="mx-auto flex max-w-[920px] flex-col gap-10 px-12 pt-12 pb-16">
      <PageTitle title="Home" subtitle={<DemoBadge areas={["requests", "agents"]} />} />
      <ComingWith card="Home and Sessions screens" />
    </div>
  );
}
