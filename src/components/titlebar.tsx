import { PanelLeft, Search, Settings as Gear } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useMessages } from "@/i18n";

/**
 * The window's title bar. The window keeps its native controls (the traffic
 * lights); mygo tells the page how much room they take in CSS variables, and
 * `--app-region: drag` makes the bar move the window. Buttons inside stay clickable.
 */
export function TitleBar({ onToggleSidebar, onSearch, onSettings }: { onToggleSidebar(): void; onSearch(): void; onSettings(): void }) {
  const t = useMessages();
  return (
    <header
      className="flex shrink-0 items-center gap-1 border-b bg-sidebar px-2 [--app-region:drag]"
      style={{
        height: "var(--mygo-titlebar-height, 40px)",
        paddingLeft: "calc(var(--mygo-titlebar-inset-left, 0px) + 0.5rem)",
        paddingRight: "calc(var(--mygo-titlebar-inset-right, 0px) + 0.5rem)",
      }}
    >
      <Button variant="ghost" size="icon-sm" title={t.toggleSidebar()} aria-label={t.toggleSidebar()} onClick={onToggleSidebar}>
        <PanelLeft />
      </Button>
      <span className="flex-1" />
      <Button variant="ghost" size="icon-sm" title={t.searchTitle()} aria-label={t.searchTitle()} onClick={onSearch}>
        <Search />
      </Button>
      <Button variant="ghost" size="icon-sm" title={t.settings()} aria-label={t.settings()} onClick={onSettings}>
        <Gear />
      </Button>
    </header>
  );
}
