import { Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";

import { ledgers } from "@/api/endpoints";
import type { Category } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { FloatingAction } from "@/components/floating-action";
import { HeaderActions } from "@/components/header-actions";
import { ListRow, ListSection } from "@/components/list";
import { Screen } from "@/components/screen";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { bookingCount, purposeLabels } from "@/lib/categories";
import { hueFor } from "@/lib/hues";
import { categoryIcon } from "@/lib/icons";
import { loadMyLedgers } from "@/lib/ledgers";
import { cn } from "@/lib/utils";

/**
 * All categories of a ledger. Every member may add one; only the owner may
 * open a category to change or delete it (rule K9), so for others the rows
 * are plain text.
 */
export default function CategoriesScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  const load = useCallback(() => Promise.all([ledgers.categories(ledgerId), loadMyLedgers()]), [ledgerId]);
  const { data, error, loading, reload } = useLoader(load);
  const [categories, myLedgers] = data ?? [[], []];
  const isOwner = myLedgers.find((l) => l.id === ledgerId)?.isOwner ?? false;

  const active = categories.filter((c) => !c.archived);
  const groups = [
    { title: "Expenses", items: active.filter((c) => !c.is_income) },
    { title: "Income", items: active.filter((c) => c.is_income) },
    { title: "Archived", items: categories.filter((c) => c.archived) },
  ];

  return (
    <Screen
      floating={
        <FloatingAction label="New category" href={{ pathname: "/ledgers/[ledgerId]/categories/new", params: { ledgerId } }} />
      }>
      <Stack.Screen options={{ title: "Categories", headerRight: () => <HeaderActions /> }} />
      {data && !isOwner && <Text variant="muted">Only the owner of the household can change or delete categories.</Text>}
      {loading && !data && <Skeleton className="h-64 w-full rounded-2xl" />}
      <ErrorMessage message={error} onRetry={reload} />
      {groups.map(
        (g) =>
          g.items.length > 0 && (
            <ListSection key={g.title} title={g.title}>
              {g.items.map((c) => (
                <ListRow
                  key={c.id}
                  icon={categoryIcon(c.name)}
                  hue={hueFor(c.id)}
                  title={c.name}
                  subtitle={details(c)}
                  className={cn(c.archived && "opacity-60")}
                  href={
                    isOwner
                      ? { pathname: "/ledgers/[ledgerId]/categories/[categoryId]", params: { ledgerId, categoryId: c.id } }
                      : undefined
                  }
                  chevron={isOwner}
                />
              ))}
            </ListSection>
          ),
      )}
    </Screen>
  );
}

function details(c: Category): string {
  return [c.purpose && purposeLabels[c.purpose], c.is_fallback && "Catch-all", bookingCount(c.transaction_count)]
    .filter(Boolean)
    .join(" · ");
}
