import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import { ClientContext, httpClient } from "./api/client";
import "./styles.css";

// The composition root: the only place that picks concrete implementations.
const root = document.getElementById("root");
if (!root) throw new Error("index.html has no #root");
createRoot(root).render(
  <StrictMode>
    <ClientContext.Provider value={httpClient()}>
      <App />
    </ClientContext.Provider>
  </StrictMode>,
);
