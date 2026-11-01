import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { ApiError, type Proposal, useClient } from "../api/client";
import { memberChanges, openerOf, soulFields, soulText } from "../app/chiefOfStaff";
import { harnessLabel, roleLabel, roster } from "../app/model";
import { cosProposalKey, keys, useCosProposal, useProfiles, useTeams } from "../app/queries";
import { TeamPlaybook } from "../components/TeamPlaybook";
import { Terminal, type TerminalHandle } from "../components/Terminal";
import { Button } from "../components/ui";

// The Chief of Staff (wiki/product/features/chief-of-staff.md#the-page): its
// terminal, where the CEO says what they want, and the preview of the team it
// proposes, where they create it. Its session runs while the page is open.

// A stop waits this long: a page that unmounts and mounts again at once (a
// new entry point, React's development double mount) keeps its session.
const stopDelay = 1000;
let pendingStop: ReturnType<typeof setTimeout> | undefined;
let lastOpener: string | undefined; // what the running session was last opened with

export function ChiefOfStaff() {
  const client = useClient();
  const queries = useQueryClient();
  const search = useSearch({ from: "/chief-of-staff" });
  const teams = useTeams();
  const proposal = useCosProposal();
  const harness = useProfiles().data?.find((p) => p.name === "default")?.harness ?? "";
  const [previewing, setPreviewing] = useState(true);
  const [saved, setSaved] = useState("");
  const term = useRef<TerminalHandle>(null);
  // The opener needs the names of the team and the member it is about.
  const opener = teams.data ? openerOf(search, teams.data) : undefined;

  const session = useMutation({
    mutationFn: (line: string) => client.chiefOfStaff.start(line),
    onSuccess: (s) => queries.setQueryData(cosProposalKey, s.proposal),
  });
  // biome-ignore lint/correctness/useExhaustiveDependencies: one start per opener
  useEffect(() => {
    if (opener === undefined) return;
    clearTimeout(pendingStop);
    // Mounted again with the same opener while its stop waited: already said.
    session.mutate(pendingStop !== undefined && lastOpener === opener ? "" : opener);
    pendingStop = undefined;
    lastOpener = opener;
    const stop = () => {
      client.chiefOfStaff.stop().catch(() => {});
      queries.setQueryData(cosProposalKey, null);
    };
    window.addEventListener("pagehide", stop);
    return () => {
      window.removeEventListener("pagehide", stop);
      pendingStop = setTimeout(() => {
        pendingStop = undefined;
        lastOpener = undefined;
        stop();
      }, stopDelay);
    };
  }, [opener]);

  // A new proposal opens the preview.
  const proposalId = proposal?.id;
  useEffect(() => {
    if (proposalId !== undefined) {
      setPreviewing(true);
      setSaved("");
    }
  }, [proposalId]);

  const terminal = session.data?.terminal;
  const error = session.error;
  const showPreview = proposal !== null && previewing;
  return (
    <div className="flex h-full min-h-0 flex-col">
      <header className="flex items-center gap-2.5 border-b border-line px-6 py-3.5">
        <span className="grid size-7 place-items-center rounded-[7px] bg-acc font-mono text-[13px] font-bold text-accInk">
          C
        </span>
        <h1 className="m-0 text-[15px] font-semibold">Chief of Staff</h1>
        <span className="font-mono text-[11.5px] text-tx4">
          {harness ? `${harnessLabel(harness)} · default` : "default"}
        </span>
        <span className="flex-1" />
        {saved && <span className="text-[13px] text-ok">✓ Saved {saved}</span>}
        {proposal && !previewing && (
          <Button variant="small" onClick={() => setPreviewing(true)}>
            Show preview
          </Button>
        )}
      </header>
      <div className="flex min-h-0 flex-1">
        <div
          className={`min-h-0 min-w-[320px] flex-1 bg-term ${showPreview ? "max-[900px]:hidden" : ""}`}
        >
          {terminal && <Terminal instance={terminal} ref={term} />}
          {error && <StartError error={error} />}
        </div>
        {showPreview && (
          <Preview
            key={proposal.id}
            proposal={proposal}
            onAdjust={() => {
              setPreviewing(false);
              term.current?.focus();
            }}
            onSaved={(name) => {
              setSaved(name);
              setPreviewing(false);
            }}
          />
        )}
      </div>
    </div>
  );
}

function StartError({ error }: { error: Error }) {
  const code = error instanceof ApiError ? error.code : "";
  return (
    <div className="flex flex-col gap-2 p-6 text-[13.5px]">
      <p className="m-0 text-err">{error.message}</p>
      {code === "NOT_CONNECTED" && (
        <Link to="/settings" className="text-accTx">
          Open Settings → Agents → Profiles
        </Link>
      )}
    </div>
  );
}

function Preview({
  proposal,
  onAdjust,
  onSaved,
}: {
  proposal: Proposal;
  onAdjust: () => void;
  onSaved: (name: string) => void;
}) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const team = proposal.team;
  const isNew = !proposal.current;
  const { members, leaving } = memberChanges(proposal);
  const ids = roster(team);
  const first = ids.find((id) => members[id]?.status !== "same") ?? ids[0];
  const [tab, setTab] = useState(first);
  const create = useMutation({
    mutationFn: () => client.chiefOfStaff.create(proposal.id),
    onSuccess: (saved) => {
      queries.invalidateQueries({ queryKey: keys.teams });
      if (isNew)
        navigate({ to: "/teams/$team", params: { team: saved.id }, search: { tab: "members" } });
      else onSaved(saved.config.name);
    },
  });
  const soul = team.agents[tab];
  const change = members[tab];
  return (
    <aside className="flex min-h-0 w-[60%] max-w-[760px] flex-col border-l border-line bg-panel max-[900px]:w-full max-[900px]:max-w-none">
      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-auto px-6 py-5">
        <div className="flex flex-col gap-1">
          <span className="text-[12px] font-medium text-tx4">
            {isNew ? "New team" : `Changes to ${proposal.current?.name}`}
          </span>
          <h2 className="m-0 text-[22px] font-semibold tracking-[-0.01em]">{team.name}</h2>
          {team.purpose && <p className="m-0 text-tx3">{team.purpose}</p>}
        </div>
        <TeamPlaybook team={team} />
        <nav className="flex flex-wrap gap-1 border-b border-line">
          {ids.map((id) => (
            <button
              key={id}
              type="button"
              onClick={() => setTab(id)}
              className={`-mb-px flex items-center gap-2 border-b-2 px-2.5 py-2 ${
                id === tab ? "border-acc text-tx" : "border-transparent text-tx3 hover:text-tx2"
              }`}
            >
              {team.agents[id].name}
              {members[id]?.status === "new" && (
                <span className="text-[11.5px] text-accTx">new</span>
              )}
              {members[id]?.status === "changed" && <span className="text-attn">•</span>}
            </button>
          ))}
        </nav>
        {soul && (
          <div className="flex flex-col gap-3.5">
            <div className="flex flex-col">
              <span className="font-semibold">{soul.name}</span>
              <span className="text-[13px] text-tx3">
                {roleLabel(soul.role)}
                {tab === team.lead ? " · lead" : ""}
              </span>
            </div>
            {soulFields
              .filter((f) => f.key !== "name" && f.key !== "role")
              .map((f) => (
                <Field
                  key={f.key}
                  label={f.label}
                  value={soulText(soul, f.key)}
                  was={change?.was[f.key]}
                />
              ))}
            {change?.was.name !== undefined && (
              <Field label="Name" value={soul.name} was={change.was.name} />
            )}
            {change?.was.role !== undefined && (
              <Field label="Role" value={roleLabel(soul.role)} was={roleLabel(change.was.role)} />
            )}
          </div>
        )}
        {leaving.length > 0 && (
          <p className="m-0 text-[13.5px] text-err">
            Leaves the team:{" "}
            {leaving.map((id) => proposal.current?.agents[id]?.name ?? id).join(", ")}
          </p>
        )}
      </div>
      <footer className="flex flex-wrap items-center gap-3 border-t border-line px-6 py-3.5">
        {create.error && (
          <span className="flex-1 text-[13px] text-err">{create.error.message}</span>
        )}
        <span className="flex-1" />
        <Button onClick={onAdjust}>Adjust team</Button>
        <Button variant="primary" disabled={create.isPending} onClick={() => create.mutate()}>
          {isNew ? `Create ${team.name}` : `Save ${team.name}`}
        </Button>
      </footer>
    </aside>
  );
}

function Field({ label, value, was }: { label: string; value: string; was?: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-[12px] font-medium text-tx4">{label}</span>
      <p className="m-0 whitespace-pre-wrap text-[13.5px] leading-[1.55] text-tx2">
        {value || (
          <span className="text-tx4">
            {label === "Job" ? "The role's default job description" : "—"}
          </span>
        )}
      </p>
      {was !== undefined && (
        <p className="m-0 whitespace-pre-wrap text-[12.5px] text-tx4 line-through">{was || "—"}</p>
      )}
    </div>
  );
}
