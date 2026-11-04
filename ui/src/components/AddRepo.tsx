import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type Project, useClient } from "../api/client";
import { defaultToSend, inside, repoName } from "../app/addProject";
import { keys, useBranches } from "../app/queries";
import { FolderBrowser, useFolderBrowser } from "./FolderBrowser";
import { Select } from "./Select";
import { Button } from "./ui";

// Adds a code repo to a project (wiki/product/features/add-project.md#picking-repos),
// in one step: the folder the browser shows is the pick. A git repo's top
// folder is added as it is; any other folder gets a new empty repo inside it.
// Used by Repos.

const label = "text-[13px] font-medium text-tx4";
const field =
  "rounded-lg border border-line2 bg-term px-3 py-2 text-[13.5px] text-tx disabled:opacity-45";
const hint = "text-[12.5px] text-tx4";

export function AddRepo({ project, onAdded }: { project: string; onAdded: (p: Project) => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const browser = useFolderBrowser(project);
  const folder = browser.folder;
  const existing = !!folder?.repo;
  // Already this project's repo: nothing to add (the server would refuse it).
  const added = folder?.added;
  // Foreman's own folder (where it started, say): no repo is added or made here.
  const reserved = folder?.foreman;
  // The name typed, for the kind of folder it was typed in: a repo's name
  // follows its folder until the CEO edits it; a new one's is typed. Moving
  // between a repo and a plain folder drops it, so neither takes the other's.
  const [typed, setTyped] = useState<{ value: string; repo: boolean }>();
  // An existing repo without a main: the branch the CEO picks as its default.
  const [chosen, setChosen] = useState<string>();
  const branches = useBranches(existing ? folder?.path : undefined);
  const defaultBranch = existing ? defaultToSend(branches.data, chosen) : undefined;
  const value =
    typed && typed.repo === existing
      ? typed.value
      : existing && folder
        ? repoName(folder.path)
        : "";
  const name = value.trim();
  const path = folder && (existing ? folder.path : name && inside(folder.path, name));
  const add = useMutation({
    mutationFn: () =>
      existing
        ? client.home.addRepo(project, name, path || "", defaultBranch)
        : client.home.newRepo(project, name, path || ""),
    onSuccess: (p) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      setTyped(undefined);
      setChosen(undefined);
      onAdded(p);
    },
  });
  return (
    <div className="flex flex-col gap-3">
      <FolderBrowser browser={browser} />
      <label className="flex flex-col gap-1.5">
        <span className={label}>Repo name</span>
        <input
          value={value}
          onChange={(e) => setTyped({ value: e.target.value, repo: existing })}
          placeholder={existing ? undefined : "my-repo"}
          spellCheck={false}
          className={field}
        />
        <span className={hint}>
          {reserved
            ? `This is ${reserved}, Foreman's own: open another folder.`
            : added
              ? `Already in this project, as ${added}.`
              : existing
                ? "This folder is a git repo: it's added as it is. Its name in this project follows the folder's unless you change it; agents see it as a folder of that name."
                : `Not a git repo: creates ${path || `${folder?.path ?? "…"}/<name>`}, a new repo with a README in a first commit, on main. Open a repo's folder above to add it instead.`}
        </span>
      </label>
      {defaultBranch !== undefined && branches.data && (
        <div className="flex flex-col gap-1.5">
          <span className={label}>Default branch</span>
          <Select
            label="Default branch"
            value={defaultBranch}
            onChange={setChosen}
            options={branches.data.branches.map((b) => ({ value: b, label: b }))}
          />
          <span className={hint}>
            This repo has no branch named main. Pick its default: agents start their branches from
            it, and merges go to it unless the lead names another target. You can change it later in
            Repos.
          </span>
        </div>
      )}
      {(add.error ?? branches.error) && (
        <p className="m-0 text-[13px] text-err">{(add.error ?? branches.error)?.message}</p>
      )}
      <div className="flex justify-end">
        <Button
          variant="primary"
          disabled={!path || !!added || !!reserved || add.isPending || branches.isLoading}
          onClick={() => add.mutate()}
        >
          {existing ? "Add repo" : "Create repo"}
        </Button>
      </div>
    </div>
  );
}
