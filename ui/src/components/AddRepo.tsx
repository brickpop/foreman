import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type Project, useClient } from "../api/client";
import { inside, mainToSend, repoName } from "../app/addProject";
import { keys, useBranches } from "../app/queries";
import { RepoPicker } from "./RepoPicker";
import { Select } from "./Select";
import { Button, Segmented } from "./ui";

// Adds a code repo to a project (wiki/product/features/add-project.md#picking-repos):
// an existing one picked from disk, or a new empty one in a folder the CEO
// picks. Used by the New project dialog and Project settings.

const label = "text-[13px] font-medium text-tx4";
const field =
  "rounded-lg border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx disabled:opacity-45";
const hint = "text-[12.5px] text-tx4";

export function AddRepo({ project, onAdded }: { project: string; onAdded: (p: Project) => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const [mode, setMode] = useState<"existing" | "new">("existing");
  // Existing: the repo's folder. New: the folder the repo goes in.
  const [picked, setPicked] = useState<string>();
  // undefined follows the picked folder's name until the CEO edits it.
  const [name, setName] = useState<string>();
  // An existing repo without a main: the branch the CEO picks as its main.
  const [chosen, setChosen] = useState<string>();
  const branches = useBranches(mode === "existing" ? picked : undefined);
  const main = mainToSend(branches.data, chosen);
  const value = name ?? (mode === "existing" && picked ? repoName(picked) : "");
  const path =
    mode === "existing" ? picked : picked && value.trim() && inside(picked, value.trim());
  const reset = () => {
    setPicked(undefined);
    setName(undefined);
    setChosen(undefined);
  };
  const add = useMutation({
    mutationFn: () =>
      mode === "existing"
        ? client.home.addRepo(project, value.trim(), path ?? "", main)
        : client.home.newRepo(project, value.trim(), path ?? ""),
    onSuccess: (p) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      reset();
      onAdded(p);
    },
  });
  return (
    <div className="flex flex-col gap-3">
      <Segmented
        label="Repo"
        value={mode}
        onChange={(m) => {
          setMode(m);
          reset();
          add.reset();
        }}
        options={[
          { value: "existing", label: "Existing repo" },
          { value: "new", label: "New empty repo" },
        ]}
      />
      {picked ? (
        <div className="flex items-center gap-2">
          <span className="min-w-0 flex-1 truncate font-mono text-[13px] text-tx2" title={picked}>
            {mode === "new" ? `in ${picked}` : picked}
          </span>
          <Button variant="small" onClick={reset}>
            Change
          </Button>
        </div>
      ) : (
        <RepoPicker pick={mode === "existing" ? "repo" : "folder"} onPick={setPicked} />
      )}
      <label className="flex flex-col gap-1.5">
        <span className={label}>Repo name</span>
        <input
          value={value}
          onChange={(e) => setName(e.target.value)}
          spellCheck={false}
          className={field}
        />
        <span className={hint}>
          {mode === "existing"
            ? "Its name in this project, named after its folder unless you change it. Agents see it as a folder of that name."
            : `Creates ${path || "<folder>/<name>"} with a README in a first commit, on main.`}
        </span>
      </label>
      {main !== undefined && branches.data && (
        <div className="flex flex-col gap-1.5">
          <span className={label}>Main branch</span>
          <Select
            label="Main branch"
            value={main}
            mono
            onChange={setChosen}
            options={branches.data.branches.map((b) => ({ value: b, label: b }))}
          />
          <span className={hint}>
            This repo has no main branch. Pick its default: agents start their branches from it, and
            merges go to it. You can change it later in Project settings.
          </span>
        </div>
      )}
      {(add.error ?? branches.error) && (
        <p className="m-0 text-[13px] text-err">{(add.error ?? branches.error)?.message}</p>
      )}
      <div className="flex justify-end">
        <Button
          variant="primary"
          disabled={!path || !value.trim() || add.isPending || branches.isLoading}
          onClick={() => add.mutate()}
        >
          {mode === "existing" ? "Add repo" : "Create repo"}
        </Button>
      </div>
    </div>
  );
}
