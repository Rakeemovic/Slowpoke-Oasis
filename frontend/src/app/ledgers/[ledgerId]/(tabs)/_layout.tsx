import { Stack, Tabs, useLocalSearchParams } from "expo-router";
import { ChartPie, Ellipsis, House, PiggyBank, type LucideIcon } from "lucide-react-native";
import { useColorScheme } from "nativewind";
import { useMemo } from "react";
import { Platform, type ColorValue } from "react-native";

import { HeaderActions } from "@/components/header-actions";
import { useLoader } from "@/hooks/use-loader";
import { LedgerProvider, type OpenLedger } from "@/lib/ledger-context";
import { loadMyLedgers } from "@/lib/ledgers";
import { THEME } from "@/lib/theme";

/**
 * A ledger is split into four tabs: what happens now (overview), the plan
 * (budget), the money set aside (pots) and everything else (more). Tabs keep
 * the main areas one tap away, as on most banking apps; screens below them
 * (a form, a pot) open on top with a back button.
 */
export default function LedgerTabsLayout() {
  const params = useLocalSearchParams<{ ledgerId: string; name?: string; kind?: string; householdId?: string }>();
  const { data } = useLoader(loadMyLedgers);
  const found = data?.find((l) => l.id === params.ledgerId);
  const { colorScheme } = useColorScheme();
  const theme = THEME[colorScheme === "light" ? "light" : "dark"];

  const ledger = useMemo<OpenLedger>(
    () => ({
      ledgerId: params.ledgerId,
      // The parameters show the name at once; the loaded list fills in what they lack after a reload.
      name: found?.name ?? params.name ?? "Ledger",
      kind: found?.kind ?? (params.kind === "personal" || params.kind === "household" ? params.kind : undefined),
      householdId: found?.householdId ?? params.householdId,
      isOwner: found?.isOwner,
    }),
    [found, params.ledgerId, params.name, params.kind, params.householdId],
  );

  return (
    <LedgerProvider value={ledger}>
      <Stack.Screen options={{ title: ledger.name, headerRight: () => <HeaderActions /> }} />
      <Tabs
        screenOptions={{
          headerShown: false,
          tabBarActiveTintColor: theme.primary,
          tabBarInactiveTintColor: theme.mutedForeground,
          tabBarStyle: {
            backgroundColor: theme.card,
            borderTopColor: theme.border,
            // On the web there is no safe area below the bar, so it gets room of its own.
            ...Platform.select({ web: { height: 64, paddingTop: 6, paddingBottom: 8 } }),
          },
          tabBarLabelStyle: { fontSize: 12, fontWeight: "600" },
        }}>
        <Tabs.Screen name="index" options={{ title: "Overview", tabBarIcon: tabIcon(House) }} />
        <Tabs.Screen name="budget" options={{ title: "Budget", tabBarIcon: tabIcon(ChartPie) }} />
        <Tabs.Screen name="pots" options={{ title: "Pots", tabBarIcon: tabIcon(PiggyBank) }} />
        <Tabs.Screen name="more" options={{ title: "More", tabBarIcon: tabIcon(Ellipsis) }} />
      </Tabs>
    </LedgerProvider>
  );
}

function tabIcon(IconComponent: LucideIcon) {
  function TabIcon({ color, focused }: { color: ColorValue; focused: boolean }) {
    return <IconComponent color={String(color)} size={22} strokeWidth={focused ? 2.4 : 2} />;
  }
  return TabIcon;
}
