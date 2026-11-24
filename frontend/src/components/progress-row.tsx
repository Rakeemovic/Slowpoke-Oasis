import type { ReactNode } from "react";
import { View } from "react-native";

import { percent, type Progress, type ProgressTone } from "@/lib/progress";
import { cn } from "@/lib/utils";

const fillColour: Record<ProgressTone, string> = {
  default: "bg-primary/15",
  done: "bg-primary/30",
  over: "bg-destructive/25",
};

/**
 * A row whose background is a progress bar: the colour fills it from the left
 * up to the progress. The row's own content (texts, amounts) stays on top, so
 * every number is still readable as text and nothing depends on colour alone.
 * Without progress the row is plain. Put it inside a Pressable to make the
 * whole bar tappable.
 *
 * The fill is hidden from screen readers: they get the same facts from the
 * row's text, and a progress bar inside a button would only garble its name.
 * `label` names the bar in the DOM (aria-label), e.g. "Groceries", which also
 * lets the end-to-end tests read the value.
 */
export function ProgressRow({
  progress,
  label,
  className,
  children,
}: {
  progress?: Progress;
  label: string;
  className?: string;
  children: ReactNode;
}) {
  return (
    <View className="relative overflow-hidden">
      {progress && (
        <View
          // The fill is decoration: taps go to the row.
          pointerEvents="none"
          aria-hidden
          aria-label={`${label} progress`}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={percent(progress)}
          className={cn("absolute inset-y-0 left-0", fillColour[progress.tone])}
          style={{ width: `${progress.fraction * 100}%` }}
        />
      )}
      <View className={className}>{children}</View>
    </View>
  );
}
