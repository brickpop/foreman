import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it } from "vitest";
import { ClientContext, httpClient, type Project } from "../api/client";
import { AddRepo } from "./AddRepo";

it("opens with Clone from a URL selected ahead of A folder", () => {
  const project: Project = {
    id: "ledger",
    name: "Ledger",
    path: "/projects/ledger/hq",
    here: true,
    remote: "",
    files: "/projects/ledger/files",
    profile: "",
    hidden: false,
    config: {
      description: "",
      checks: "",
      alwaysAskCeo: [],
      recursiveSubmodules: false,
      landingPrefixes: [],
    },
    repos: [],
    version: "",
    problems: [],
    source: "",
  };
  const html = renderToStaticMarkup(
    <ClientContext.Provider value={httpClient("")}>
      <QueryClientProvider client={new QueryClient()}>
        <AddRepo project={project} onAdded={() => {}} />
      </QueryClientProvider>
    </ClientContext.Provider>,
  );

  expect(html).toMatch(
    /<button[^>]*aria-checked="true"[^>]*>Clone from a URL<\/button><button[^>]*aria-checked="false"[^>]*>A folder<\/button>/,
  );
  expect(html).toMatch(/<label[^>]*>URL<\/label>/);
});
