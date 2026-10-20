import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./style.css";

// The webview's own context menu (Reload, Inspect Element) is not part of the
// app. Text fields and selected text keep it for Copy and Paste; the recording
// list shows its own menu.
window.addEventListener("contextmenu", (e) => {
  const el = e.target as HTMLElement;
  const editable = el.closest("input, textarea, [contenteditable=true]");
  if (!editable && !window.getSelection()?.toString()) e.preventDefault();
});

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
