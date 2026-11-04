import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { type Suggestion, useClient } from "../api/client";
import { createdBody, projectSlug } from "../app/addProject";
import { readable } from "../app/model";
import { keys } from "../app/queries";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button, Segmented } from "./ui";

// The New project dialog (wiki/product/features/add-project.md): a name, its
// workspace and its project repo (New, or Cloned with the CEO's own git). Once
// created, the project's Repos page opens, to add its code repos.

const label = "text-[13px] font-medium text-tx4";
const field =
  "rounded-lg border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx disabled:opacity-45";
const hint = "text-[12.5px] text-tx4";

export function NewProject({ onClose }: { onClose: () => void }) {
  return (
    <Modal open onClose={onClose} title="New project">
      <Create onClose={onClose} />
    </Modal>
  );
}

function Create({ onClose }: { onClose: () => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const [name, setName] = useState("");
  // What is sent: the name as a slug, the project's id and folder.
  const id = projectSlug(name);
  const [source, setSource] = useState<"new" | "clone">("new");
  const [clone, setClone] = useState("");
  // The workspace follows the name, in the projects folder, until the CEO
  // edits it.
  const [edited, setEdited] = useState<string>();
  const suggestion = useQuery({
    queryKey: ["workspace", id],
    queryFn: () => client.home.suggestWorkspace(id),
    placeholderData: (previous) => previous,
  }).data;
  const workspace = edited ?? (id ? (suggestion?.path ?? "") : "");
  const create = useMutation({
    mutationFn: () =>
      client.home.newProject(id, workspace.trim(), source === "clone" ? clone.trim() : undefined),
    onSuccess: ({ project: p, created }) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      onClose();
      navigate({ to: "/projects/$project/repos", params: { project: p.id } });
      toast({
        title: `${readable(p.id)} created`,
        body: createdBody(created, source === "clone", p.path),
        color: "var(--ok)",
      });
    },
  });
  const ready = id && workspace.trim() && (source === "new" || clone.trim());
  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault();
        if (ready && !create.isPending) create.mutate();
      }}
    >
      <label className="flex flex-col gap-1.5">
        <span className={label}>Project name</span>
        <input
          value={name}
          onChange={(e) => setName(e.target.value)}
          spellCheck={false}
          className={field}
        />
        <span className={hint}>
          {id
            ? `Its folder and id: ${id}. It shows as ${readable(id)}.`
            : "Any words: they become its folder, in lowercase with hyphens."}
        </span>
      </label>
      <label className="flex flex-col gap-1.5">
        <span className={label}>Workspace</span>
        <input
          value={workspace}
          onChange={(e) => setEdited(e.target.value)}
          placeholder={suggestion ? `${suggestion.folder}/<name>` : undefined}
          spellCheck={false}
          className={field}
        />
        <span className={hint}>{workspaceHint(workspace, suggestion)}</span>
      </label>
      <div className="flex flex-col gap-1.5">
        <span className={label}>Project repo</span>
        <Segmented
          label="Project repo"
          value={source}
          onChange={setSource}
          options={[
            { value: "new", label: "New" },
            { value: "clone", label: "Clone" },
          ]}
        />
        {source === "clone" && (
          <input
            value={clone}
            onChange={(e) => setClone(e.target.value)}
            placeholder="git@github.com:you/ledger-project.git, or a local path"
            aria-label="Clone from"
            spellCheck={false}
            className={field}
          />
        )}
        <span className={hint}>
          {source === "new"
            ? "Its wiki and project.toml, in a new git repo: the workspace's project/ folder. One left there by a removed project is taken over as it is."
            : "Cloned with your own git: your SSH keys and credential helper. Foreman stores nothing."}
        </span>
      </div>
      {create.error && <p className="m-0 text-[13px] text-err">{create.error.message}</p>}
      <div className="flex justify-end gap-2">
        <Button onClick={onClose}>Cancel</Button>
        <Button type="submit" variant="primary" disabled={!ready || create.isPending}>
          {create.isPending && source === "clone" ? "Cloning…" : "Create project"}
        </Button>
      </div>
    </form>
  );
}

/** What the workspace field says: what goes there, and that the projects
 * folder is created with it when it doesn't exist yet. */
function workspaceHint(workspace: string, s: Suggestion | undefined): string {
  const what =
    "Your folder for this project: its project repo and its agents' worktrees, where you can run their code. Anywhere in your home folder.";
  if (s && !s.folderExists && workspace.startsWith(`${s.folder}/`)) {
    return `${what} ${s.folder} doesn't exist yet: it is created with the project.`;
  }
  return what;
}
