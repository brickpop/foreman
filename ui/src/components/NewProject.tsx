import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useClient } from "../api/client";
import { createdBody, newProjectHint, projectSlug } from "../app/addProject";
import { projectName } from "../app/model";
import { keys } from "../app/queries";
import { Modal } from "./Overlays";
import { useToast } from "./Toasts";
import { Button, Segmented } from "./ui";

// The New project dialog (wiki/product/features/add-project.md): a name and
// its HQ (New, or Cloned with the CEO's own git). Once created, the project's
// Repos page opens, to add its code repos.

const label = "text-[13px] font-medium text-tx4";
const field =
  "rounded-lg border border-line2 bg-term px-3 py-2 font-mono text-[13px] text-tx disabled:opacity-45";
const hint = "text-[12.5px] text-tx4 [overflow-wrap:anywhere]";

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
  // What is sent: the name as a slug (the project's id and folder) and the
  // name itself, trimmed (how it shows).
  const id = projectSlug(name);
  const [source, setSource] = useState<"new" | "clone">("new");
  const [clone, setClone] = useState("");
  const create = useMutation({
    mutationFn: () =>
      client.home.newProject(id, name.trim(), source === "clone" ? clone.trim() : undefined),
    onSuccess: ({ project: p, created }) => {
      queries.invalidateQueries({ queryKey: keys.projects });
      onClose();
      navigate({ to: "/projects/$project/repos", params: { project: p.id } });
      toast({
        title: `${projectName([p], p.id)} created`,
        body: createdBody(created, source === "clone", p.path),
        color: "var(--ok)",
      });
    },
  });
  const ready = id && (source === "new" || clone.trim());
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
          onChange={(e) => {
            setName(e.target.value);
            if (create.isError) create.reset();
          }}
          maxLength={80}
          spellCheck={false}
          className={field}
        />
        <span className={hint}>{newProjectHint(name)}</span>
      </label>
      <div className="flex flex-col gap-1.5">
        <span className={label}>HQ</span>
        <Segmented
          label="HQ"
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
            onChange={(e) => {
              setClone(e.target.value);
              if (create.isError) create.reset();
            }}
            placeholder="git@github.com:you/ledger-hq.git, or a local path"
            aria-label="Clone from"
            spellCheck={false}
            className={field}
          />
        )}
        <span className={hint}>
          {source === "new"
            ? "The project’s wiki and project.toml, in a new git repo: its hq/ folder. One left there by a removed project is taken over as it is."
            : "Cloned with your own git: your SSH keys and credential helper. Foreman stores nothing."}
        </span>
      </div>
      {create.error && (
        <p role="alert" className="m-0 text-[13px] text-err">
          {create.error.message}
        </p>
      )}
      <div className="flex justify-end gap-2">
        <Button onClick={onClose}>Cancel</Button>
        <Button type="submit" variant="primary" disabled={!ready || create.isPending}>
          {create.isPending && source === "clone" ? "Cloning…" : "Create project"}
        </Button>
      </div>
    </form>
  );
}
