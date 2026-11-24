/** How a progress fill is coloured: normal, finished, or past the limit. */
export type ProgressTone = "default" | "done" | "over";

export interface Progress {
  /** How much of the row is filled, from 0 to 1. */
  fraction: number;
  tone: ProgressTone;
}

/**
 * Spent against a budgeted amount. The fill stops at the full row; spending
 * more than planned turns it to the warning colour. Without a plan there is
 * nothing to measure against, so there is no fill.
 */
export function spentProgress(actual: number, planned: number | null): Progress | undefined {
  if (!planned || planned <= 0) {
    return undefined;
  }
  return { fraction: clamp(actual / planned), tone: actual > planned ? "over" : "default" };
}

/** Received or saved against an amount to reach: reaching it is the good end, not a warning. */
export function reachedProgress(actual: number, wanted: number | null): Progress | undefined {
  if (!wanted || wanted <= 0) {
    return undefined;
  }
  return { fraction: clamp(actual / wanted), tone: actual >= wanted ? "done" : "default" };
}

/** A whole number for screen readers, e.g. 0.304 → 30. */
export function percent(progress: Progress): number {
  return Math.round(progress.fraction * 100);
}

function clamp(value: number): number {
  return Math.min(1, Math.max(0, value));
}
