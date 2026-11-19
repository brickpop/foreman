import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type Project, useClient } from "../api/client";
import {
  anyWords,
  cloneName,
  defaultToSend,
  inside,
  nameFromFolder,
  repoIdLine,
} from "../app/addProject";
import { repoName, repoNameRefusal, slug } from "../app/model";
import { keys, useBranches } from "../app/queries";
import { FolderBrowser, useFolderBrowser } from "./FolderBrowser";
import { Select } from "./Select";
import { Button, Field, Segmented } from "./ui";

// Adds a code repo to a project (wiki/product/features/add-project.md#picking-repos):
// from a folder, in one step (the folder the browser shows is the pick: a git
// repo's top folder is added as it is; any other folder gets a new empty
// repo inside it), or cloned from a URL into Foreman's repos/. Used by Repos.
// The repo's id is made from its Name by the server; the hint shows it live,
// and the clashes the server would refuse show before Add
// (spec/repo-display-names-ux.md).

const field =
  "rounded-lg border border-line2 bg-term px-3 py-2 text-[13.5px] text-tx disabled:opacity-45";

type Props = { project: Project; onAdded: (p: Project) => void };

export function AddRepo(props: Props) {
  const [from, setFrom] = useState<"folder" | "clone">("folder");
  return (
    <div className="flex flex-col gap-3">
      <Segmented
        label="Add from"
        value={from}
        onChange={setFrom}
        options={[
          { value: "folder", label: "A folder" },
          { value: "clone", label: "Clone from a URL" },
        ]}
      />
      {from === "folder" ? <FromFolder {...props} /> : <FromClone {...props} />}
    </div>
  );
}

/** Clones a repo with the CEO's git into Foreman's repos/<the URL's path>;
 * the server picks its default branch (main, else the remote's). */
function FromClone({ project, onAdded }: Props) {
  const client = useClient();
  const queries = useQueryClient();
  const [url, setUrl] = useState("");
  const [typed, setTyped] = useState<string>();
  const value = typed ?? cloneName(url);
  const id = slug(value);
  const refusal = repoNameRefusal(project, value);
  const add = useMutation({
    mutationFn: () => client.home.cloneRepo(project.id, value.trim(), url.trim()),
    onSuccess: (p) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      setUrl("");
      setTyped(undefined);
      onAdded(p);
    },
  });
  const changed = () => add.isError && add.reset();
  return (
    <div className="flex flex-col gap-3">
      <Field
        label="URL"
        hint="Cloned with your git (your SSH keys, credential helper) into Foreman's folder, repos/ and the URL's path (me/ledger-api). A clone already there is used as it is. Its default branch is main, or the remote's when it has no main."
      >
        {(input) => (
          <input
            {...input}
            value={url}
            onChange={(e) => {
              setUrl(e.target.value);
              changed();
            }}
            placeholder="git@github.com:me/ledger-api.git"
            spellCheck={false}
            className={`${field} font-mono`}
          />
        )}
      </Field>
      <Field label="Name" hint={repoIdLine(value) ?? anyWords} refusal={refusal}>
        {(input) => (
          <input
            {...input}
            value={value}
            onChange={(e) => {
              setTyped(e.target.value);
              changed();
            }}
            maxLength={80}
            spellCheck={false}
            className={field}
          />
        )}
      </Field>
      <ServerError error={add.error} />
      <div className="flex justify-end">
        <Button
          variant="primary"
          disabled={!url.trim() || !id || !!refusal || add.isPending}
          onClick={() => add.mutate()}
        >
          {add.isPending ? "Cloning…" : "Clone repo"}
        </Button>
      </div>
    </div>
  );
}

function FromFolder({ project, onAdded }: Props) {
  const client = useClient();
  const queries = useQueryClient();
  const browser = useFolderBrowser(project.id);
  const folder = browser.folder;
  const existing = !!folder?.repo;
  // Already this project's repo (its id): nothing to add (the server would refuse it).
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
        ? nameFromFolder(folder.path)
        : "";
  const id = slug(value);
  // A new repo's folder is named by its id.
  const path = folder && (existing ? folder.path : id && inside(folder.path, id));
  const refusal = added || reserved ? undefined : repoNameRefusal(project, value);
  // The folder each Add was for: its refusal shows only while that folder is shown.
  const add = useMutation({
    mutationFn: (_folder: string) =>
      existing
        ? client.home.addRepo(project.id, value.trim(), path || "", defaultBranch)
        : client.home.newRepo(project.id, value.trim(), path || ""),
    onSuccess: (p) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      setTyped(undefined);
      setChosen(undefined);
      onAdded(p);
    },
  });
  const line = repoIdLine(value);
  const hint = reserved
    ? `This is ${reserved}, Foreman's own: open another folder.`
    : added
      ? `Already in this project, as ${repoName(project, added)}.`
      : !line
        ? anyWords
        : existing
          ? `${line} The folder keeps its name; agents see the repo as ${id}.`
          : `${line} Not a git repo: creates ${path}, a new repo with a README in a first commit, on main. Open a repo's folder above to add it instead.`;
  return (
    <div className="flex flex-col gap-3">
      <FolderBrowser browser={browser} />
      <Field label="Name" hint={hint} refusal={refusal}>
        {(input) => (
          <input
            {...input}
            value={value}
            onChange={(e) => {
              setTyped({ value: e.target.value, repo: existing });
              if (add.isError) add.reset();
            }}
            placeholder={existing ? undefined : "My repo"}
            maxLength={80}
            spellCheck={false}
            className={field}
          />
        )}
      </Field>
      {defaultBranch !== undefined && branches.data && (
        <Field
          label="Default branch"
          hint="This repo has no branch named main. Pick its default: agents start their branches from it, and merges go to it unless the lead names another target. You can change it later in Repos."
        >
          {(input) => (
            <Select
              field={input}
              label="Default branch"
              value={defaultBranch}
              onChange={setChosen}
              options={branches.data.branches.map((b) => ({ value: b, label: b }))}
            />
          )}
        </Field>
      )}
      <ServerError error={(add.variables === folder?.path ? add.error : null) ?? branches.error} />
      <div className="flex justify-end">
        <Button
          variant="primary"
          disabled={
            !path || !!added || !!reserved || !!refusal || add.isPending || branches.isLoading
          }
          onClick={() => folder && add.mutate(folder.path)}
        >
          {existing ? "Add repo" : "Create repo"}
        </Button>
      </div>
    </div>
  );
}

/** A refusal from the server, over the form's button. */
function ServerError({ error }: { error: Error | null }) {
  if (!error) return null;
  return (
    <p role="alert" className="m-0 text-[13px] text-err">
      {error.message}
    </p>
  );
}
