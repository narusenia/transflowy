import { cn } from "@/lib/utils";

/** Text with a light sweeping across it, for work in progress: shadcn's `shimmer` utility (shadcn/tailwind.css). */
export function Shimmer({ children, className }: { children: string; className?: string }) {
  return <span className={cn("shimmer inline-block text-muted-foreground", className)}>{children}</span>;
}
