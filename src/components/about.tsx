import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { useMessages } from "@/i18n";
import { About } from "@/mygo";
import icon from "../../resources/icon.png";

/** Name, version, license and the licenses of what the app includes. */
export function AboutDialog({ open, onOpenChange }: { open: boolean; onOpenChange(o: boolean): void }) {
  const t = useMessages();
  const [version, setVersion] = useState("");
  const [notices, setNotices] = useState<string | null>(null);
  const [showing, setShowing] = useState(false);

  const change = (o: boolean) => {
    if (o) About.version().then(setVersion).catch(() => setVersion(""));
    else setShowing(false);
    onOpenChange(o);
  };
  // ~100 KB: loaded when the list is first opened, not with the page.
  const toggle = () => {
    if (notices === null) void import("../../THIRD_PARTY_NOTICES.md?raw").then((m) => setNotices(m.default));
    setShowing(!showing);
  };

  return (
    <Dialog open={open} onOpenChange={change}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader className="items-center text-center">
          <img src={icon} alt="" className="size-20" />
          <DialogTitle>transflowy</DialogTitle>
          <DialogDescription>{version ? t.aboutVersion(version) : t.aboutDevBuild()}</DialogDescription>
        </DialogHeader>
        <p className="text-center text-xs text-muted-foreground">{t.aboutLicense()}</p>
        <div className="flex justify-center gap-2">
          <Button variant="outline" size="sm" onClick={() => void About.openRepository()}>
            {t.aboutRepository()}
          </Button>
          <Button variant="outline" size="sm" onClick={toggle}>
            {showing ? t.aboutNoticesHide() : t.aboutNotices()}
          </Button>
        </div>
        {showing && (
          <pre className="max-h-64 overflow-auto rounded-md border bg-muted/40 p-3 text-[10px] leading-relaxed whitespace-pre-wrap select-text">
            {notices ?? ""}
          </pre>
        )}
      </DialogContent>
    </Dialog>
  );
}
