import { Link, useSearch } from "@tanstack/react-router";
import { useSettings } from "../app/queries";
import { settingsSearch, settingsTabs } from "../app/settings";
import { PageTitle } from "../components/ui";
import { AgentsTab } from "./settings/AgentsTab";
import { GitTab } from "./settings/GitTab";
import { LookTab } from "./settings/LookTab";
import { SystemTab } from "./settings/SystemTab";

// Settings (wiki/product/features/settings.md), in tabs by topic, one file
// each under settings/. Each section says where it is kept: the global repo's
// settings.toml, the same on every machine you clone it to, or this machine
// only. Every change saves at once.

export function Settings() {
  const tab = useSearch({ from: "/settings" }).tab ?? "agents";
  const file = useSettings();
  const s = file.data?.settings;
  return (
    <div className="mx-auto flex w-full max-w-[760px] flex-col gap-9 px-12 pt-12 pb-16">
      <div className="flex flex-col gap-4">
        <PageTitle title="Settings" />
        <nav className="flex flex-wrap border-b border-line">
          {settingsTabs.map((t) => (
            <Link
              key={t.tab}
              to="/settings"
              search={settingsSearch({ tab: t.tab })}
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
      {tab === "agents" && <AgentsTab settings={s} />}
      {tab === "git" && s && <GitTab settings={s} />}
      {tab === "look" && <LookTab settings={s} />}
      {tab === "system" && <SystemTab settings={s} />}
    </div>
  );
}
