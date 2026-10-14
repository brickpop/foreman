import { DemoBadge } from "../components/DemoBadge";
import { PageTitle } from "../components/ui";
import { ComingWith } from "./Placeholder";

// wiki/product/features/settings.md
export function Settings() {
  return (
    <div className="mx-auto flex max-w-[760px] flex-col gap-9 px-12 pt-12 pb-16">
      <PageTitle title="Settings" subtitle={<DemoBadge areas={["workspace", "harnesses"]} />} />
      <ComingWith card="Settings screen, notifications and toasts" />
    </div>
  );
}
