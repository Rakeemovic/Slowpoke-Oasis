import { Link, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { pots } from "@/api/endpoints";
import type { Pot } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { HeaderActions } from "@/components/header-actions";
import { ProgressRow } from "@/components/progress-row";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { describeGoal, isGoal, potProgress } from "@/lib/goals";
import { formatCents } from "@/lib/money";
import { isReserve } from "@/lib/reserves";
import { cn } from "@/lib/utils";

export default function PotsScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  const { data, error, loading, reload } = useLoader(useCallback(() => pots.list(ledgerId), [ledgerId]));
  const active = data?.filter((p) => !p.archived) ?? [];
  // Reserves save for recurring payments due every few months (rule R10);
  // goals are pots with a target or a monthly amount.
  const goals = active.filter(isGoal);
  const own = active.filter((p) => !isReserve(p) && !isGoal(p));
  const reserves = active.filter(isReserve);
  const archived = data?.filter((p) => p.archived) ?? [];

  return (
    <Screen>
      <Stack.Screen options={{ title: "Pots", headerRight: () => <HeaderActions /> }} />
      <View className="flex-row gap-2">
        <Link href={{ pathname: "/ledgers/[ledgerId]/pots/new", params: { ledgerId } }} asChild>
          <Button className="flex-1">
            <Text>New pot</Text>
          </Button>
        </Link>
        <Link href={{ pathname: "/ledgers/[ledgerId]/move", params: { ledgerId } }} asChild>
          <Button className="flex-1" variant="outline">
            <Text>Move between pots</Text>
          </Button>
        </Link>
      </View>
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      {own.length > 0 && <PotList ledgerId={ledgerId} pots={own} />}
      {goals.length > 0 && (
        <>
          <Text variant="muted">Goals</Text>
          <PotList ledgerId={ledgerId} pots={goals} />
        </>
      )}
      {reserves.length > 0 && (
        <>
          <Text variant="muted">Reserves</Text>
          <PotList ledgerId={ledgerId} pots={reserves} />
        </>
      )}
      {archived.length > 0 && (
        <>
          <Text variant="muted">Archived</Text>
          <PotList ledgerId={ledgerId} pots={archived} />
        </>
      )}
    </Screen>
  );
}

function PotList({ ledgerId, pots: items }: { ledgerId: string; pots: Pot[] }) {
  return (
    <Card className="py-2">
      <CardContent className="gap-0 px-0">
        {items.map((pot, index) => (
          <Link key={pot.id} href={{ pathname: "/ledgers/[ledgerId]/pots/[potId]", params: { ledgerId, potId: pot.id } }} asChild>
            <Pressable className="active:bg-accent web:hover:bg-accent/50" role="link">
              <PotRow pot={pot} first={index === 0} />
            </Pressable>
          </Link>
        ))}
      </CardContent>
    </Card>
  );
}

/** A goal's row is its own progress bar: filled up to the balance against the target. */
function PotRow({ pot, first }: { pot: Pot; first: boolean }) {
  const reached = pot.target_cents !== undefined && pot.balance_cents >= pot.target_cents;
  return (
    <ProgressRow
      progress={isGoal(pot) ? potProgress(pot) : undefined}
      label={pot.name}
      className={cn("flex-row items-center gap-3 px-4 py-3", !first && "border-t border-border")}>
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{pot.name}</Text>
        {pot.is_default && <Text variant="muted">Default pot, shown as “available”</Text>}
        {isGoal(pot) && <Text variant="muted">{describeGoal(pot)}</Text>}
        {pot.is_separate_account && <Text variant="muted">Separate account at the bank</Text>}
      </View>
      <View className="items-end">
        <Text className={cn("font-semibold", pot.balance_cents < 0 && "text-destructive")}>{formatCents(pot.balance_cents)}</Text>
        {isGoal(pot) && reached && <Text variant="muted" className="text-sm">Goal reached</Text>}
      </View>
    </ProgressRow>
  );
}
