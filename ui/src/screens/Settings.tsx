import { Link, Outlet, useParams } from "@tanstack/react-router";
import { useSettings } from "../app/queries";
import { settingsTab, settingsTabs } from "../app/settings";
import { TeamGrid } from "../components/TeamGrid";
import { PageTitle } from "../components/ui";
import { AgentsTab } from "./settings/AgentsTab";
import { LookTab } from "./settings/LookTab";
import { SystemTab } from "./settings/SystemTab";

// Settings (wiki/product/features/settings.md), in tabs by topic, one file
// each under settings/, and the teams' pages, each a path under it. Each
// section says where it is kept: the global repo's settings.toml, the same on
// every machine you clone it to, or this machine only. Every change saves at
// once.

/** Every Settings page's frame: one width, tabs and team pages alike. */
export function SettingsFrame() {
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-9 px-6 pt-6 pb-12">
      <Outlet />
    </div>
  );
}

export function Settings() {
  const tab = settingsTab(useParams({ from: "/settings/$tab" }).tab);
  const file = useSettings();
  const s = file.data?.settings;
  return (
    <>
      <div className="flex flex-col gap-4">
        <PageTitle title="Settings" />
        <nav className="flex flex-wrap border-b border-line">
          {settingsTabs.map((t) => (
            <Link
              key={t.tab}
              to="/settings/$tab"
              params={{ tab: t.tab }}
              className={`-mb-px border-b-2 px-3.5 py-[9px] font-medium ${
                t.tab === tab ? "border-acc text-tx" : "border-transparent text-tx3 hover:text-tx2"
              }`}
            >
              {t.label}
            </Link>
          ))}
        </nav>
      </div>
      {file.data && file.data.problems.length > 0 && (
        <p className="m-0 rounded-lg border border-err px-3 py-2 text-[13px] text-err">
          settings.toml: {file.data.problems.join(" · ")}
        </p>
      )}
      {tab === "teams" && <TeamGrid />}
      {tab === "agents" && <AgentsTab settings={s} />}
      {tab === "look" && <LookTab settings={s} />}
      {tab === "system" && <SystemTab settings={s} />}
    </>
  );
}

/** Where a team's or an agent's page is: Settings › Teams › <team> (› <agent>),
 * each step a link but the last. */
export function Crumbs({ team, agent }: { team: { id: string; name: string }; agent?: string }) {
  const step = "text-tx3 hover:text-tx";
  return (
    <nav className="flex flex-wrap items-center gap-1.5 text-[13.5px] text-tx4">
      <Link to="/settings" className={step}>
        Settings
      </Link>
      ›
      <Link to="/settings/$tab" params={{ tab: "teams" }} className={step}>
        Teams
      </Link>
      ›
      {agent ? (
        <>
          <Link to="/settings/teams/$team" params={{ team: team.id }} className={step}>
            {team.name}
          </Link>
          ›<span className="text-tx2">{agent}</span>
        </>
      ) : (
        <span className="text-tx2">{team.name}</span>
      )}
    </nav>
  );
}
