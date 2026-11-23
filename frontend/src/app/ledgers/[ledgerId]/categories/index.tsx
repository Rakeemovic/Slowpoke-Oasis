import { Link, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { Category } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { HeaderActions } from "@/components/header-actions";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { bookingCount, purposeLabels } from "@/lib/categories";
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
    <Screen>
      <Stack.Screen options={{ title: "Categories", headerRight: () => <HeaderActions /> }} />
      <Link href={{ pathname: "/ledgers/[ledgerId]/categories/new", params: { ledgerId } }} asChild>
        <Button>
          <Text>New category</Text>
        </Button>
      </Link>
      {data && !isOwner && <Text variant="muted">Only the owner of the household can change or delete categories.</Text>}
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      {groups.map(
        (g) =>
          g.items.length > 0 && (
            <View key={g.title} className="gap-2">
              <Text variant="muted">{g.title}</Text>
              <CategoryList ledgerId={ledgerId} categories={g.items} linked={isOwner} />
            </View>
          ),
      )}
    </Screen>
  );
}

function CategoryList({ ledgerId, categories, linked }: { ledgerId: string; categories: Category[]; linked: boolean }) {
  return (
    <Card className="py-2">
      <CardContent className="gap-0 px-0">
        {categories.map((c, index) => {
          const row = <CategoryRow category={c} first={index === 0} />;
          if (!linked) {
            return <View key={c.id}>{row}</View>;
          }
          return (
            <Link
              key={c.id}
              href={{ pathname: "/ledgers/[ledgerId]/categories/[categoryId]", params: { ledgerId, categoryId: c.id } }}
              asChild>
              <Pressable className="active:bg-accent web:hover:bg-accent/50" role="link">
                {row}
              </Pressable>
            </Link>
          );
        })}
      </CardContent>
    </Card>
  );
}

function CategoryRow({ category: c, first }: { category: Category; first: boolean }) {
  const details = [c.purpose && purposeLabels[c.purpose], c.is_fallback && "Catch-all", bookingCount(c.transaction_count)]
    .filter(Boolean)
    .join(" · ");
  return (
    <View className={cn("gap-0.5 px-4 py-3", !first && "border-t border-border")}>
      <Text className="font-medium">{c.name}</Text>
      <Text variant="muted">{details}</Text>
    </View>
  );
}
