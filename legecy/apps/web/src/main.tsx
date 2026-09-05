import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "@app/App";

import "./index.css";

const rootEl = document.getElementById("root");
if (!rootEl) {
  throw new Error("root element missing — check apps/web/index.html");
}

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
