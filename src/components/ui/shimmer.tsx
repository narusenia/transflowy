import type { CSSProperties } from "react";
import { cn } from "@/lib/utils";

/** Text with a light sweeping across it, for work in progress. The sweep stops with reduced motion. */
export function Shimmer({ children, className }: { children: string; className?: string }) {
  return (
    <span className={cn("shimmer inline-block", className)} style={{ "--spread": `${children.length * 2}px` } as CSSProperties}>
      {children}
    </span>
  );
}
