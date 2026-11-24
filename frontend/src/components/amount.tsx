import type { ComponentProps } from "react";

import { NumberTicker } from "@/components/magic/number-ticker";
import { Text } from "@/components/ui/text";
import { formatCents, formatSignedCents } from "@/lib/money";
import { cn } from "@/lib/utils";

const SIZES = {
  sm: "text-sm font-semibold",
  md: "text-base font-semibold",
  lg: "font-display text-2xl",
  xl: "font-display text-4xl tracking-tight",
};

/**
 * A money amount. Digits have equal widths (tabular figures), so amounts in a
 * column line up and do not jitter while counting. Only money coming in is
 * coloured: expenses are the normal case and stay in the text colour, which
 * keeps red for real problems such as an overspent budget.
 *
 * `signed` shows +/−; `animate` counts towards the value (large balances only).
 */
export function Amount({
  cents,
  signed = false,
  size = "md",
  tone,
  animate = false,
  className,
  ...textProps
}: {
  cents: number;
  signed?: boolean;
  size?: keyof typeof SIZES;
  /** Overrides the automatic colour. */
  tone?: "default" | "positive" | "negative" | "muted";
  animate?: boolean;
} & Omit<ComponentProps<typeof Text>, "children">) {
  const resolved = tone ?? (signed && cents > 0 ? "positive" : "default");
  const classes = cn(
    SIZES[size],
    resolved === "positive" && "text-positive",
    resolved === "negative" && "text-destructive",
    resolved === "muted" && "text-muted-foreground",
    className,
  );
  const format = signed ? formatSignedCents : formatCents;
  const style = { fontVariant: ["tabular-nums" as const] };
  if (animate) {
    return <NumberTicker value={cents} format={format} className={classes} style={style} {...textProps} />;
  }
  return (
    <Text className={classes} style={style} {...textProps}>
      {format(cents)}
    </Text>
  );
}
