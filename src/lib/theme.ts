import { useEffect, useState } from "react";

export type Mode = "system" | "light" | "dark";
const KEY = "transflowy.theme";
const query = () => matchMedia("(prefers-color-scheme: dark)");

export function getMode(): Mode {
  try {
    const m = localStorage.getItem(KEY);
    if (m === "light" || m === "dark") return m;
  } catch {
    // storage can be unavailable: follow the system
  }
  return "system";
}

function apply(mode: Mode) {
  const dark = mode === "dark" || (mode === "system" && query().matches);
  document.documentElement.classList.toggle("dark", dark);
  document.documentElement.style.colorScheme = dark ? "dark" : "light";
}

export function setMode(mode: Mode) {
  try {
    if (mode === "system") localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, mode);
  } catch {
    // the choice then lasts until the page closes
  }
  apply(mode);
}

/** Call once before the first render, so the page does not flash light. */
export function initTheme() {
  apply(getMode());
  query().addEventListener("change", () => apply(getMode()));
}

/** Whether the page is dark now, for what CSS variables cannot reach (canvas, toasts). */
export function useDark() {
  const [dark, setDark] = useState(() => document.documentElement.classList.contains("dark"));
  useEffect(() => {
    const el = document.documentElement;
    const obs = new MutationObserver(() => setDark(el.classList.contains("dark")));
    obs.observe(el, { attributes: true, attributeFilter: ["class"] });
    return () => obs.disconnect();
  }, []);
  return dark;
}
