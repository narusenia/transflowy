import { Info, PanelLeft, Search, Settings as Gear } from "lucide-react";
import { IconButton } from "@/components/icon-button";
import { useMessages } from "@/i18n";

/**
 * The window's title bar. The window keeps its native controls (the traffic
 * lights); mygo tells the page how much room they take in CSS variables, and
 * `--app-region: drag` makes the bar move the window. Buttons inside stay clickable.
 */
export function TitleBar({ onToggleSidebar, onSearch, onSettings, onAbout }: { onToggleSidebar(): void; onSearch(): void; onSettings(): void; onAbout(): void }) {
  const t = useMessages();
  return (
    <header
      className="flex shrink-0 items-center gap-1 border-b bg-sidebar px-2 [--app-region:drag]"
      style={{
        height: 40, // the traffic lights are centered in 40 px (TrafficLightPosition in internal/app/app.go)
        paddingLeft: "calc(var(--mygo-titlebar-inset-left, 0px) + 0.5rem)",
        paddingRight: "calc(var(--mygo-titlebar-inset-right, 0px) + 0.5rem)",
      }}
    >
      <IconButton variant="ghost" label={t.toggleSidebar()} onClick={onToggleSidebar}>
        <PanelLeft />
      </IconButton>
      <span className="flex-1" />
      <IconButton variant="ghost" label={t.searchTitle()} onClick={onSearch}>
        <Search />
      </IconButton>
      <IconButton variant="ghost" label={t.settings()} onClick={onSettings}>
        <Gear />
      </IconButton>
      <IconButton variant="ghost" label={t.about()} onClick={onAbout}>
        <Info />
      </IconButton>
    </header>
  );
}
