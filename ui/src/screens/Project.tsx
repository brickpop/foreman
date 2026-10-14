import { Link, useParams, useSearch } from "@tanstack/react-router";
import { DemoBadge } from "../components/DemoBadge";
import { ComingWith } from "./Placeholder";

export type ProjectTab = "overview" | "changes" | "board" | "settings";

const tabs: { tab: ProjectTab; label: string; card: string }[] = [
  { tab: "overview", label: "Overview", card: "Project pages" },
  { tab: "changes", label: "Changes", card: "Project pages" },
  { tab: "board", label: "Board", card: "Project pages" },
  { tab: "settings", label: "Settings", card: "Project pages" },
];

// wiki/product/features/wiki-reader.md#project-header-and-tabs
export function Project() {
  const { project } = useParams({ from: "/projects/$project" });
  const { tab } = useSearch({ from: "/projects/$project" });
  const current = tabs.find((t) => t.tab === tab) ?? tabs[0];
  return (
    <div className="flex flex-col">
      <header className="flex flex-col gap-4 px-12 pt-10">
        <div className="flex items-center gap-3">
          <h1 className="text-[30px] font-semibold tracking-[-0.02em]">{project}</h1>
          <DemoBadge areas={["workspace", "wiki", "pipeline", "requests"]} />
        </div>
        <nav className="flex border-b border-line">
          {tabs.map((t) => (
            <Link
              key={t.tab}
              to="/projects/$project"
              params={{ project }}
              search={{ tab: t.tab }}
              className={`-mb-px border-b-2 px-3.5 py-[9px] font-medium ${
                t.tab === tab ? "border-acc text-tx" : "border-transparent text-tx3 hover:text-tx2"
              }`}
            >
              {t.label}
            </Link>
          ))}
        </nav>
      </header>
      <div className="px-8 pt-6 pb-16">
        <ComingWith card={current.card}>The {current.label} tab</ComingWith>
      </div>
    </div>
  );
}
