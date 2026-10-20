import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useClient } from "../api/client";
import type { Endpoint } from "../api/types/harness";
import { harnessLabel } from "../app/model";
import { keys, useHarnesses, useProfiles } from "../app/queries";
import { Modal } from "./Overlays";
import { Select } from "./Select";
import { Terminal } from "./Terminal";
import { Button } from "./ui";

// A profile's credentials (wiki/product/features/settings.md#profiles), from
// Settings, the setup checklist and the Deploy dialog: the harness's own login
// in a terminal (Claude Code), or a model server in a form (Pi). Mount it
// only while open: for a login, opening starts it and closing ends it,
// logged in or not.

export function ConnectDialog({ profile, onClose }: { profile: string; onClose: () => void }) {
  const harness = useProfiles().data?.find((p) => p.name === profile)?.harness;
  const connect = useHarnesses().data?.find((h) => h.id === harness)?.connect;
  if (!connect) return null; // until both are known: a login must not start for a form
  return connect === "endpoint" ? (
    <EndpointConnect profile={profile} onClose={onClose} />
  ) : (
    <LoginConnect profile={profile} onClose={onClose} />
  );
}

function LoginConnect({ profile, onClose }: { profile: string; onClose: () => void }) {
  const client = useClient();
  const queries = useQueryClient();
  const login = useMutation({ mutationFn: () => client.profiles.connect(profile) });
  // biome-ignore lint/correctness/useExhaustiveDependencies: one login per opening
  useEffect(() => login.mutate(), []);
  // Followed while open, so the dialog says when the login landed.
  const status = useQuery({
    queryKey: keys.profiles,
    queryFn: () => client.profiles.list(),
    refetchInterval: 2000,
  });
  const found = status.data?.find((p) => p.name === profile);
  const connected = found?.connected ?? false;
  const close = () => {
    client.profiles.endLogin(profile).catch(() => {}); // already gone is fine
    queries.invalidateQueries({ queryKey: keys.profiles });
    queries.invalidateQueries({ queryKey: keys.setup });
    onClose();
  };
  const label = harnessLabel(found?.harness ?? "");
  return (
    <Modal open onClose={close} title={`Connect ${profile}`} width={760} buttonsOnly>
      <div className="flex flex-col gap-3">
        <p className="m-0 text-[13.5px] text-tx3">
          {label}’s own login, in its container. For Claude Code, type{" "}
          <span className="font-mono text-tx2">/login</span> and follow it. Every project on the{" "}
          <span className="font-mono text-tx2">{profile}</span> profile uses it.
        </p>
        <div className="h-[380px] overflow-hidden rounded-[10px] border border-line bg-term">
          {login.data && <Terminal instance={login.data} />}
        </div>
        {login.error && <p className="m-0 text-[13px] text-err">{login.error.message}</p>}
        <div className="flex items-center justify-end gap-3">
          <span className={`flex-1 text-[13px] ${connected ? "text-ok" : "text-tx4"}`}>
            {connected ? `${profile} is connected.` : "Not connected yet."}
          </span>
          <Button variant={connected ? "primary" : "secondary"} onClick={close}>
            {connected ? "Done" : "Cancel"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

/** A Pi profile's model server and model. */
function EndpointConnect({ profile, onClose }: { profile: string; onClose: () => void }) {
  const client = useClient();
  const current = useQuery({
    queryKey: [...keys.profiles, profile, "endpoint"],
    queryFn: () => client.profiles.endpoint(profile),
  });
  return (
    <Modal open onClose={onClose} title={`Connect ${profile}`} width={560} buttonsOnly>
      {current.error && <p className="m-0 text-[13px] text-err">{current.error.message}</p>}
      {current.data && <EndpointForm profile={profile} current={current.data} onClose={onClose} />}
    </Modal>
  );
}

/** The sizes a new endpoint starts with: a 256k context, replies up to 128k. */
const endpointDefaults = { contextWindow: 262144, maxTokens: 131072 };

const field =
  "rounded-[7px] border border-line2 bg-transparent px-2.5 py-1.5 font-mono text-[13px] text-tx";

function EndpointForm({
  profile,
  current,
  onClose,
}: {
  profile: string;
  current: Endpoint;
  onClose: () => void;
}) {
  const client = useClient();
  const queries = useQueryClient();
  const [url, setUrl] = useState(current.url);
  const [apiKey, setApiKey] = useState("");
  const [model, setModel] = useState(current.model);
  const [contextWindow, setContextWindow] = useState(
    current.contextWindow || endpointDefaults.contextWindow,
  );
  const [maxTokens, setMaxTokens] = useState(current.maxTokens || endpointDefaults.maxTokens);
  const detect = useMutation({
    mutationFn: () => client.profiles.models(profile, url.trim(), apiKey),
    onSuccess: (ids) => {
      if (!ids.includes(model) && ids.length > 0) setModel(ids[0]);
    },
  });
  const save = useMutation({
    mutationFn: () =>
      client.profiles.setEndpoint(profile, {
        url: url.trim(),
        apiKey,
        model: model.trim(),
        contextWindow,
        maxTokens,
      }),
    onSuccess: () => {
      queries.invalidateQueries({ queryKey: keys.profiles });
      queries.invalidateQueries({ queryKey: keys.setup });
      onClose();
    },
  });
  const detected = detect.data ?? [];
  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate();
      }}
    >
      <p className="m-0 text-[13.5px] text-tx3">
        A server with an OpenAI-compatible API (Ollama, LM Studio, llama.cpp, vLLM…) and the model
        Pi runs on it. A server on this machine is reached from the agents’ containers for you.
      </p>
      <label className="flex flex-col gap-1 text-[13px] text-tx3">
        Server URL
        <span className="flex gap-2">
          <input
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="http://localhost:11434/v1"
            className={`${field} min-w-0 flex-1`}
          />
          <Button
            variant="secondary"
            disabled={detect.isPending || url.trim() === ""}
            onClick={() => detect.mutate()}
          >
            Detect models
          </Button>
        </span>
      </label>
      <label className="flex flex-col gap-1 text-[13px] text-tx3">
        API key (optional; stored on this machine, never shown again)
        <input
          type="password"
          autoComplete="off"
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          placeholder={current.apiKeySet ? "set: leave empty to keep it" : "none"}
          className={field}
        />
      </label>
      {detect.error && <p className="m-0 text-[13px] text-err">{detect.error.message}</p>}
      {detect.isSuccess && detected.length === 0 && (
        <p className="m-0 text-[13px] text-tx3">The server offers no model: type its id.</p>
      )}
      <div className="flex flex-col gap-1 text-[13px] text-tx3">
        Model
        {detected.length > 0 ? (
          <Select
            label="Model"
            mono
            value={model}
            options={detected.map((id) => ({ value: id, label: id }))}
            onChange={setModel}
          />
        ) : (
          <input
            aria-label="Model"
            value={model}
            onChange={(e) => setModel(e.target.value)}
            placeholder="Detect, or type its id"
            className={field}
          />
        )}
      </div>
      <div className="grid grid-cols-2 gap-3">
        <label className="flex flex-col gap-1 text-[13px] text-tx3">
          Context window (tokens)
          <input
            type="number"
            min={1}
            value={contextWindow}
            onChange={(e) => setContextWindow(Number(e.target.value))}
            className={field}
          />
        </label>
        <label className="flex flex-col gap-1 text-[13px] text-tx3">
          Longest reply (tokens)
          <input
            type="number"
            min={1}
            value={maxTokens}
            onChange={(e) => setMaxTokens(Number(e.target.value))}
            className={field}
          />
        </label>
      </div>
      <p className="m-0 text-[12.5px] text-tx4">
        Saved as Pi’s <span className="font-mono">models.json</span> in the profile’s folder. Fields
        added there by hand (sampling, thinking levels) are kept while the model stays the same.
      </p>
      {save.error && <p className="m-0 text-[13px] text-err">{save.error.message}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>
          Cancel
        </Button>
        <Button
          type="submit"
          variant="primary"
          disabled={save.isPending || url.trim() === "" || model.trim() === ""}
        >
          Save
        </Button>
      </div>
    </form>
  );
}
