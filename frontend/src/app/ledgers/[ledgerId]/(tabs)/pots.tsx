import { Link } from "expo-router";
import { useCallback } from "react";
import { Pressable, View } from "react-native";

import { pots } from "@/api/endpoints";
import type { Pot } from "@/api/types";
import { Amount } from "@/components/amount";
import { ErrorMessage } from "@/components/error-message";
import { FloatingAction } from "@/components/floating-action";
import { IconTile } from "@/components/icon-tile";
import { ListSection } from "@/components/list";
import { ProgressRow } from "@/components/progress-row";
import { Screen } from "@/components/screen";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { describeGoal, isGoal, potProgress } from "@/lib/goals";
import { hueFor, MAIN_HUE } from "@/lib/hues";
import { potIcon } from "@/lib/icons";
import { useLedger } from "@/lib/ledger-context";
import { isReserve } from "@/lib/reserves";
import { describeRoundUp } from "@/lib/round-ups";
import { cn } from "@/lib/utils";

export default function PotsScreen() {
  const { ledgerId } = useLedger();
  const { data, error, loading, reload } = useLoader(useCallback(() => pots.list(ledgerId), [ledgerId]));
  const active = data?.filter((p) => !p.archived) ?? [];
  // Reserves save for recurring payments due every few months (rule R10);
  // goals are pots with a target or a monthly amount.
  const goals = active.filter(isGoal);
  const own = active.filter((p) => !isReserve(p) && !isGoal(p));
  const reserves = active.filter(isReserve);
  const archived = data?.filter((p) => p.archived) ?? [];

  return (
    <Screen
      floating={<FloatingAction label="New pot" href={{ pathname: "/ledgers/[ledgerId]/pots/new", params: { ledgerId } }} />}>
      <Text variant="muted">
        Pots split your money by purpose: what is left to spend, what you save for, and reserves for bills that come every few
        months.
      </Text>
      {loading && !data && <Skeleton className="h-40 w-full rounded-2xl" />}
      <ErrorMessage message={error} onRetry={reload} />
      {own.length > 0 && <PotList ledgerId={ledgerId} title="Pots" pots={own} />}
      {goals.length > 0 && <PotList ledgerId={ledgerId} title="Goals" pots={goals} />}
      {reserves.length > 0 && <PotList ledgerId={ledgerId} title="Reserves" pots={reserves} />}
      {archived.length > 0 && <PotList ledgerId={ledgerId} title="Archived" pots={archived} />}
    </Screen>
  );
}

function PotList({ ledgerId, title, pots: items }: { ledgerId: string; title: string; pots: Pot[] }) {
  return (
    <ListSection title={title}>
      {items.map((pot) => (
        <Link key={pot.id} href={{ pathname: "/ledgers/[ledgerId]/pots/[potId]", params: { ledgerId, potId: pot.id } }} asChild>
          <Pressable className="active:bg-accent web:hover:bg-accent/50" role="link">
            <PotRow pot={pot} />
          </Pressable>
        </Link>
      ))}
    </ListSection>
  );
}

/** A goal's row is its own progress bar: filled up to the balance against the target. */
function PotRow({ pot }: { pot: Pot }) {
  const reached = pot.target_cents !== undefined && pot.balance_cents >= pot.target_cents;
  const roundUp = describeRoundUp(pot);
  const goal = isGoal(pot);
  return (
    <ProgressRow
      progress={goal ? potProgress(pot) : undefined}
      label={pot.name}
      className={cn("min-h-16 flex-row items-center gap-3 px-4 py-3", pot.archived && "opacity-60")}>
      <IconTile icon={potIcon(pot, goal)} hue={pot.is_default ? MAIN_HUE : hueFor(pot.id)} />
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{pot.name}</Text>
        {pot.is_default && <Text variant="muted">Everyday money, free to spend</Text>}
        {goal && <Text variant="muted">{describeGoal(pot)}</Text>}
        {pot.is_separate_account && <Text variant="muted">Separate account at the bank</Text>}
        {roundUp && <Text variant="muted">{roundUp}</Text>}
      </View>
      <View className="items-end">
        <Amount cents={pot.balance_cents} tone={pot.balance_cents < 0 ? "negative" : "default"} />
        {goal && reached && <Text className="text-xs font-medium text-positive">Goal reached</Text>}
      </View>
    </ProgressRow>
  );
}
