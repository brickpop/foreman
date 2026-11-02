import { useNavigate, useParams } from "@tanstack/react-router";
import { useEffect, useRef, useState } from "react";
import { isConflict, type Soul, type Team } from "../api/client";
import { useOpenChiefOfStaff } from "../app/chiefOfStaff";
import { readable, roleLabel } from "../app/model";
import { sessionOf } from "../app/project";
import { useInstances, usePrompt, useSaveSoul, useTeams } from "../app/queries";
import { useToast } from "../components/Toasts";
import { Button, Dot, Loading } from "../components/ui";
import { Crumbs } from "./Settings";

// One agent's soul (wiki/product/features/agent-page.md): who it is, and
// the exact prompt each deployment gives it (what it runs on is its project's
// profile). No save button: choices save at once, text on blur or Enter.

const option = (on: boolean) =>
  `rounded-[6px] border px-2.5 py-1 text-[13px] ${on ? "border-acc bg-accSoft text-tx" : "border-line2 text-tx2 hover:border-line3"}`;

export function Agent() {
  const { team: teamId, agent: id } = useParams({ from: "/settings/teams/$team/agents/$agent" });
  const teams = useTeams();
  const team = teams.data?.find((t) => t.id === teamId);
  const soul = team?.config.agents[id];
  if (teams.isPending) return <Loading rows={5} />;
  if (!team || !soul)
    return (
      <p className="text-tx3">
        No agent {id} on team {teamId}.
      </p>
    );
  return (
    <div className="flex flex-col gap-6">
      <Crumbs team={{ id: team.id, name: team.config.name }} agent={soul.name} />
      <Editor key={`${team.id}/${id}`} team={team} id={id} saved={soul} />
      <PromptView team={team} id={id} />
    </div>
  );
}

function Editor({ team, id, saved }: { team: Team; id: string; saved: Soul }) {
  const navigate = useNavigate();
  const instances = useInstances();
  const openChiefOfStaff = useOpenChiefOfStaff();
  const teamsQuery = useTeams();
  const toast = useToast();
  const soul = useSaveSoul(team, id);
  const [draft, setDraft] = useState(saved);
  const [savedAt, setSavedAt] = useState(0);
  const [conflict, setConflict] = useState(false);
  const [skill, setSkill] = useState("");
  // The soul the draft started from: a draft that differs holds unsaved typing.
  const base = useRef(saved);
  // A save from elsewhere shows at once, unless it would drop unsaved typing:
  // then the CEO chooses (Keep mine / Take theirs). Our own saves change nothing.
  // biome-ignore lint/correctness/useExhaustiveDependencies: follow the saved soul's version only
  useEffect(() => {
    if (team.version === soul.lastSaved()) return;
    const typing = JSON.stringify(draft) !== JSON.stringify(base.current);
    base.current = saved;
    if (typing) setConflict(true);
    else setDraft(saved);
  }, [team.version]);

  const savedTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const commit = (next: Soul) => {
    setDraft(next);
    soul.save(next).then(
      (s) => {
        base.current = s.value;
        setConflict(false);
        setSavedAt(Date.now());
        clearTimeout(savedTimer.current);
        savedTimer.current = setTimeout(() => setSavedAt(0), 3000);
      },
      (e: Error) => {
        if (isConflict(e)) setConflict(true);
        else toast({ title: "Not saved", body: e.message, color: "var(--err)" });
      },
    );
  };
  const addSkill = () => {
    const s = skill.trim();
    setSkill("");
    if (s && !draft.skills.includes(s)) set("skills", [...draft.skills, s]);
  };
  const set = <K extends keyof Soul>(key: K, value: Soul[K]) => commit({ ...draft, [key]: value });
  const running = (instances.data ?? []).filter((i) => i.id.startsWith(`${id}@`));
  const savedNote = savedAt > 0 && (
    <span className="text-[12.5px] text-ok">✓ Saved. Applies on every project.</span>
  );

  return (
    <div className="flex flex-col gap-7">
      <header className="flex items-center gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="m-0 text-[26px] font-semibold tracking-[-0.02em]">{draft.name}</h1>
          <p className="m-0 text-tx3">{roleLabel(draft.role)}</p>
        </div>
      </header>

      {conflict && (
        <div className="flex flex-col gap-2 rounded-xl border border-attn bg-attnSoft p-3.5">
          <span className="text-[13.5px]">
            The team file changed elsewhere since you opened it. Your edit is still here.
          </span>
          <span className="flex gap-2">
            <Button
              variant="primary"
              onClick={async () => {
                // Save over the latest version: read it first.
                const fresh = await teamsRefetch();
                if (fresh) soul.seen(fresh);
                commit(draft);
              }}
            >
              Keep mine
            </Button>
            <Button
              onClick={() => {
                setConflict(false);
                setDraft(saved);
                base.current = saved;
              }}
            >
              Take theirs
            </Button>
          </span>
        </div>
      )}

      <Section label="Running on">
        {running.length === 0 ? (
          <p className="m-0 text-tx3">
            Not running. {team.config.name} isn't deployed to any project.
          </p>
        ) : (
          running.map((i) => (
            <div
              key={i.id}
              className="grid grid-cols-[auto_auto_1fr_auto] items-center gap-3 rounded-[9px] border border-line bg-panel px-3 py-2"
            >
              <Dot status={i.status} size={8} />
              <span className="whitespace-nowrap font-semibold">
                {readable(i.id.split("@")[1])}
              </span>
              <span className="truncate text-[13.5px] text-tx2">{i.task}</span>
              <Button variant="small" onClick={() => navigate(sessionOf(i.id))}>
                Session
              </Button>
            </div>
          ))
        )}
      </Section>

      <Section label="Personality">
        <textarea
          rows={3}
          value={draft.personality}
          onChange={(e) => setDraft({ ...draft, personality: e.target.value })}
          onBlur={() => draft.personality !== saved.personality && commit(draft)}
          aria-label="Personality"
          className={`${field} text-[15px]`}
        />
      </Section>

      <Section label="Skills">
        <div className="flex flex-wrap items-center gap-1.5">
          {draft.skills.map((s) => (
            <span
              key={s}
              className="flex items-center gap-1 rounded-[6px] bg-raised py-1 pr-1.5 pl-2.5 text-[13px] text-tx2"
            >
              {s}
              <button
                type="button"
                aria-label={`Remove ${s}`}
                onClick={() =>
                  set(
                    "skills",
                    draft.skills.filter((x) => x !== s),
                  )
                }
                className="text-tx4 hover:text-tx2"
              >
                ×
              </button>
            </span>
          ))}
          <input
            value={skill}
            onChange={(e) => setSkill(e.target.value)}
            // Enter adds it, and so does leaving the field: a typed skill is never lost.
            onKeyDown={(e) => e.key === "Enter" && addSkill()}
            onBlur={addSkill}
            placeholder="+ add"
            aria-label="Add a skill"
            className="w-20 rounded-[6px] border border-dashed border-line2 bg-transparent px-2 py-1 text-[13px] text-tx placeholder:text-tx4"
          />
        </div>
      </Section>

      <Section label="Job description" aside={savedNote}>
        <textarea
          rows={6}
          value={draft.instruction}
          onChange={(e) => setDraft({ ...draft, instruction: e.target.value })}
          onBlur={() => draft.instruction !== saved.instruction && commit(draft)}
          aria-label="Job description"
          className={`${field} bg-term font-mono text-[13px]`}
        />
      </Section>

      <div>
        <Button onClick={() => openChiefOfStaff({ start: "refine", team: team.id, agent: id })}>
          Refine with Chief of Staff
        </Button>
      </div>
    </div>
  );

  // Keep mine saves over the latest version: fetch it first.
  async function teamsRefetch(): Promise<Team | undefined> {
    const r = await teamsQuery.refetch();
    return r.data?.find((t) => t.id === team.id);
  }
}

const field =
  "w-full resize-y rounded-[8px] border border-line2 bg-term px-3 py-2 text-tx placeholder:text-tx4";

function Section({
  label,
  aside,
  children,
}: {
  label: string;
  aside?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <section className="flex flex-col gap-2.5">
      <div className="flex items-center justify-between">
        <h2 className="m-0 text-[13px] font-medium text-tx4">{label}</h2>
        {aside}
      </div>
      {children}
    </section>
  );
}

function PromptView({ team, id }: { team: Team; id: string }) {
  const instances = useInstances();
  const running = (instances.data ?? []).filter((i) => i.id.startsWith(`${id}@`));
  const [chosen, setChosen] = useState<string>();
  const instance = running.find((i) => i.id === chosen)?.id ?? running[0]?.id;
  const prompt = usePrompt(team.id, id, instance);
  return (
    <div className="flex flex-col gap-3">
      <h2 className="m-0 text-[15px] font-semibold">Session prompt</h2>
      <p className="m-0 text-[13.5px] text-tx3">
        Generated at session start from this soul plus the project. The soul is the same everywhere;
        the context isn't.
      </p>
      {running.length > 1 && (
        <div className="flex flex-wrap gap-1.5">
          {running.map((i) => (
            <button
              key={i.id}
              type="button"
              aria-pressed={i.id === instance}
              onClick={() => setChosen(i.id)}
              className={option(i.id === instance)}
            >
              on {readable(i.id.split("@")[1])}
            </button>
          ))}
        </div>
      )}
      <pre className="m-0 overflow-x-auto rounded-[10px] border border-line bg-term p-3.5 font-mono text-[12.5px] leading-[1.65] whitespace-pre-wrap text-tx2">
        {instance ? prompt.data : highlight(prompt.data ?? "")}
      </pre>
    </div>
  );
}

/** The template's {placeholders}, highlighted. */
function highlight(text: string): React.ReactNode[] {
  return text.split(/(\{[a-z.]+\})/).map((part, i) =>
    /^\{[a-z.]+\}$/.test(part) ? (
      // biome-ignore lint/suspicious/noArrayIndexKey: the parts of one fixed string
      <span key={i} className="rounded-[3px] bg-accSoft px-0.5 text-accTx">
        {part}
      </span>
    ) : (
      part
    ),
  );
}
