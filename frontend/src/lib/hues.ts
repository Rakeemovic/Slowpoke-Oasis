/**
 * A small palette of playful hues for categories, pots and ledgers. Items
 * have no stored colour, so the hue is derived from the item's ID: the same
 * item always gets the same colour, on every device and in every list.
 *
 * Each entry holds complete Tailwind class names (Tailwind only finds classes
 * written out in full) for a soft tile background and a strong foreground,
 * readable in both schemes.
 */
export interface Hue {
  tile: string;
  icon: string;
  fill: string;
}

const HUES: Hue[] = [
  { tile: "bg-teal-500/15", icon: "text-teal-700 dark:text-teal-300", fill: "bg-teal-500" },
  { tile: "bg-orange-500/15", icon: "text-orange-700 dark:text-orange-300", fill: "bg-orange-500" },
  { tile: "bg-violet-500/15", icon: "text-violet-700 dark:text-violet-300", fill: "bg-violet-500" },
  { tile: "bg-sky-500/15", icon: "text-sky-700 dark:text-sky-300", fill: "bg-sky-500" },
  { tile: "bg-pink-500/15", icon: "text-pink-700 dark:text-pink-300", fill: "bg-pink-500" },
  { tile: "bg-amber-500/20", icon: "text-amber-700 dark:text-amber-300", fill: "bg-amber-500" },
  { tile: "bg-lime-500/20", icon: "text-lime-700 dark:text-lime-300", fill: "bg-lime-500" },
  { tile: "bg-indigo-500/15", icon: "text-indigo-700 dark:text-indigo-300", fill: "bg-indigo-500" },
];

/** A neutral hue for things that are not a single item, such as a transfer. */
export const NEUTRAL_HUE: Hue = { tile: "bg-muted", icon: "text-muted-foreground", fill: "bg-muted-foreground" };

/** The main pot (the account itself) always wears the app's own colour. */
export const MAIN_HUE: Hue = { tile: "bg-primary/15", icon: "text-primary", fill: "bg-primary" };

/** The hue of an item, picked by a simple string hash of its ID. */
export function hueFor(id: string | undefined): Hue {
  if (!id) {
    return NEUTRAL_HUE;
  }
  let hash = 0;
  for (let i = 0; i < id.length; i++) {
    hash = (hash * 31 + id.charCodeAt(i)) | 0;
  }
  return HUES[Math.abs(hash) % HUES.length];
}
