import type { ComponentProps } from "react";
import { Button } from "@/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

/** An icon-only button. The label is its tooltip and its accessible name. */
export function IconButton({ label, side, size = "icon-sm", ...props }: ComponentProps<typeof Button> & { label: string; side?: "top" | "bottom" | "left" | "right" }) {
  return (
    <Tooltip>
      <TooltipTrigger render={<Button size={size} aria-label={label} {...props} />} />
      <TooltipContent side={side ?? "bottom"}>{label}</TooltipContent>
    </Tooltip>
  );
}
