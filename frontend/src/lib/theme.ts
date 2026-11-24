// Colours for places that cannot use Tailwind classes, such as the navigation
// header, the tab bar and icons. Generated from src/global.css; keep in sync.
import { DarkTheme, DefaultTheme, type Theme } from "expo-router/react-navigation";

export const THEME = {
  light: {
    background: "hsl(42 45% 96%)",
    foreground: "hsl(200 35% 12%)",
    card: "hsl(0 0% 100%)",
    cardForeground: "hsl(200 35% 12%)",
    popover: "hsl(0 0% 100%)",
    popoverForeground: "hsl(200 35% 12%)",
    primary: "hsl(174 82% 29%)",
    primaryForeground: "hsl(0 0% 100%)",
    secondary: "hsl(40 55% 90%)",
    secondaryForeground: "hsl(28 45% 22%)",
    muted: "hsl(40 30% 92%)",
    mutedForeground: "hsl(200 12% 38%)",
    accent: "hsl(172 55% 91%)",
    accentForeground: "hsl(174 82% 20%)",
    destructive: "hsl(356 72% 50%)",
    positive: "hsl(152 70% 30%)",
    warning: "hsl(32 95% 42%)",
    border: "hsl(38 25% 86%)",
    input: "hsl(38 22% 80%)",
    ring: "hsl(174 82% 35%)",
  },
  dark: {
    background: "hsl(200 32% 6%)",
    foreground: "hsl(42 35% 94%)",
    card: "hsl(199 28% 10%)",
    cardForeground: "hsl(42 35% 94%)",
    popover: "hsl(199 28% 10%)",
    popoverForeground: "hsl(42 35% 94%)",
    primary: "hsl(170 72% 48%)",
    primaryForeground: "hsl(200 45% 7%)",
    secondary: "hsl(199 22% 16%)",
    secondaryForeground: "hsl(42 35% 94%)",
    muted: "hsl(199 22% 14%)",
    mutedForeground: "hsl(200 12% 66%)",
    accent: "hsl(172 40% 16%)",
    accentForeground: "hsl(170 70% 72%)",
    destructive: "hsl(356 80% 64%)",
    positive: "hsl(152 60% 55%)",
    warning: "hsl(38 95% 60%)",
    border: "hsl(199 22% 18%)",
    input: "hsl(199 20% 24%)",
    ring: "hsl(170 72% 48%)",
  },
};

export type Scheme = keyof typeof THEME;

export const NAV_THEME: Record<Scheme, Theme> = {
  light: navTheme(DefaultTheme, "light"),
  dark: navTheme(DarkTheme, "dark"),
};

function navTheme(base: Theme, scheme: Scheme): Theme {
  const t = THEME[scheme];
  return {
    ...base,
    colors: {
      background: t.background,
      border: t.border,
      card: t.background,
      notification: t.destructive,
      primary: t.primary,
      text: t.foreground,
    },
  };
}
