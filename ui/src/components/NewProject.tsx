import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { type Project, type Suggestion, useClient } from "../api/client";
import { createdBody } from "../app/addProject";
import { keys } from "../app/queries";
import { AddRepo } from "./AddRepo";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button, Segmented } from "./ui";

// The New project dialog (wiki/product/features/add-project.md): a name, its
// workspace and its project repo (New, or Cloned with the CEO's own git), then
// its code repos.

const label = "text-[13px] font-medium text-tx4";
const field =
  "rounded-lg border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx disabled:opacity-45";
const hint = "text-[12.5px] text-tx4";

export function NewProject({ onClose }: { onClose: () => void }) {
  const [project, setProject] = useState<Project>();
  return (
    <Modal open onClose={onClose} title={project ? `Repos of ${project.id}` : "New project"}>
      {project ? (
        <Repos project={project} onClose={onClose} />
      ) : (
        <Create onCreated={setProject} onClose={onClose} />
      )}
    </Modal>
  );
}

function Create({ onCreated, onClose }: { onCreated: (p: Project) => void; onClose: () => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const navigate = useNavigate();
  const toast = useToast();
  const [name, setName] = useState("");
  const [source, setSource] = useState<"new" | "clone">("new");
  const [clone, setClone] = useState("");
  // The workspace follows the name, in the projects folder, until the CEO
  // edits it.
  const [edited, setEdited] = useState<string>();
  const suggestion = useQuery({
    queryKey: ["workspace", name.trim()],
    queryFn: () => client.home.suggestWorkspace(name.trim()),
    placeholderData: (previous) => previous,
  }).data;
  const workspace = edited ?? (name.trim() ? (suggestion?.path ?? "") : "");
  const create = useMutation({
    mutationFn: () =>
      client.home.newProject(
        name.trim(),
        workspace.trim(),
        source === "clone" ? clone.trim() : undefined,
      ),
    onSuccess: ({ project: p, created }) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      navigate({
        to: "/projects/$project",
        params: { project: p.id },
        search: { tab: "overview" },
      });
      toast({
        title: `${p.id} created`,
        body: createdBody(created, source === "clone", p.path),
        color: "var(--ok)",
      });
      onCreated(p);
    },
  });
  const ready = name.trim() && workspace.trim() && (source === "new" || clone.trim());
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
          Letters, digits, dots, hyphens and underscores, starting with a letter or digit.
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

function Repos({ project, onClose }: { project: Project; onClose: () => void }) {
  const [repos, setRepos] = useState(project.repos.map((r) => r.name));
  return (
    <div className="flex flex-col gap-4">
      <p className="m-0 text-[13.5px] text-tx2">
        {repos.length === 0
          ? "Agents work on code repos: add at least one, here or later in the project's Settings."
          : `Added: ${repos.join(", ")}. Add another, or finish.`}
      </p>
      <AddRepo project={project.id} onAdded={(p) => setRepos(p.repos.map((r) => r.name))} />
      <div className="flex justify-end border-t border-line pt-4">
        <Button onClick={onClose}>{repos.length === 0 ? "Later" : "Done"}</Button>
      </div>
    </div>
  );
}
