# UI guidelines

How the app looks and why, so new screens fit in. The building blocks live in
`frontend/src/components/`; this page says when to use which.

## Principles

These follow established guidance rather than taste alone: Apple's Human
Interface Guidelines, Material Design 3, Nielsen's usability heuristics and
"Refactoring UI" (Wathan, Schoger).

- **One main action per screen.** It is the filled button, or the floating
  button at the bottom right on list screens (`FloatingAction`). Everything
  else is outlined, ghost or a list row.
- **Hierarchy by size and weight, not by colour.** The display font (Nunito)
  is only for headings and large amounts. Secondary text is
  `text-muted-foreground`, small section titles use `Text variant="overline"`.
- **Colour has meaning.** Turquoise (`primary`) marks what can be pressed or
  is the app's own; green (`positive`) is money coming in; red
  (`destructive`) is a problem (overspent, overdue, a destructive action);
  amber (`warning`) asks for attention. Expenses stay in the text colour,
  because spending is normal, not an error. Nothing relies on colour alone:
  there is always a text or an icon as well.
- **Every item has a face.** Categories, pots and members get an icon tile in
  a hue derived from their ID (`src/lib/hues.ts`), so lists can be scanned
  without reading. Icons of the default categories: `src/lib/icons.ts`.
- **Group related things** in a `ListSection` with a title, like iOS settings.
  Lists of bookings are grouped by day ("Today", "Yesterday", …).
- **Touch targets** are at least 44 px high (buttons `h-12`, inputs `h-12`,
  rows `min-h-14`).
- **Amounts** use `Amount`: tabular figures, so columns line up.
- **Nothing is ever blank.** Loading shows `Skeleton` placeholders of the
  final size; empty lists show an `EmptyState` that says what would be here
  and how to add it; errors show `ErrorMessage` with "Try again".
- **Feedback after actions.** Saving, deleting or restoring confirms with a
  short toast (`toast.success` from sonner-native) at the top.
- **Progressive disclosure.** Rarely used inputs appear on request (e.g.
  "New category" in the booking form), rarely used screens sit under the
  "More" tab.
- **Motion explains, never decorates.** Only the large balance counts up
  (`NumberTicker`), lists fade in (`BlurFade`) and goal rings fill
  (`CircularProgress`). All of it is skipped when the system asks for reduced
  motion.

## Navigation

A ledger has four tabs: **Overview** (balance, safe to spend, quick actions,
due payments, bookings), **Budget**, **Pots** and **More** (transfer, move,
reconcile, recurring payments, categories, members, deleted bookings).
Forms and details open on top of the tabs with a back button.

## Libraries

- Components: shadcn via React Native Reusables (`src/components/ui/`).
- Icons: lucide-react-native.
- Toasts: sonner-native (the React Native port of Sonner).
- Motion: React Native Reanimated. Magic UI components are web-only, so a few
  are ported in `src/components/magic/`.
