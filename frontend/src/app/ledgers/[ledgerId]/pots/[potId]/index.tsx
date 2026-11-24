import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";
import { toast } from "sonner-native";

import { auth, ledgers, pots } from "@/api/endpoints";
import type { Pot, SavingsGoal } from "@/api/types";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { PotForm, type PotFormValues } from "@/components/pot-form";
import { ProgressRow } from "@/components/progress-row";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { describeGoal, describeRate, isGoal, potProgress } from "@/lib/goals";
import { isReserve } from "@/lib/reserves";
import { canCollectRoundUps, collectingPot } from "@/lib/round-ups";
import { formatCents } from "@/lib/money";

export default function PotScreen() {
  const { ledgerId, potId } = useLocalSearchParams<{ ledgerId: string; potId: string }>();
  const { data, error, loading, reload } = useLoader(
    useCallback(async () => {
      const [list, goals, me] = await Promise.all([pots.list(ledgerId), ledgers.savingsGoals(ledgerId, today()), auth.me()]);
      // Round-ups exist only in the personal ledger (rule T10).
      return { list, goals, isPersonal: me.personal_ledger_id === ledgerId };
    }, [ledgerId]),
  );
  const pot = data?.list.find((p) => p.id === potId);
  const goal = data?.goals.find((g) => g.pot_id === potId);

  if (!pot) {
    return (
      <Screen>
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error ?? (data ? "This pot does not exist any more." : undefined)} onRetry={reload} />
      </Screen>
    );
  }

  async function save(values: PotFormValues) {
    // round_up_multiplier is only part of values when the form offered it.
    await pots.update(ledgerId, potId, { ...values, archived: pot!.archived }, today());
    toast.success("Pot saved");
    router.back();
  }
  const collecting = collectingPot(data!.list);

  return (
    <Screen>
      {isGoal(pot) && !pot.archived && <GoalSummary pot={pot} goal={goal} />}
      <PotForm
        initial={{
          name: pot.name,
          target_cents: pot.target_cents ?? 0,
          target_date: pot.target_date ?? "",
          monthly_rate_cents: pot.monthly_rate_cents ?? 0,
          is_separate_account: pot.is_separate_account,
          round_up_multiplier: pot.round_up_multiplier,
        }}
        hideAccount={pot.is_default || isReserve(pot)}
        hideGoal={pot.is_default}
        roundUp={
          canCollectRoundUps(pot, data!.isPersonal)
            ? { replaces: collecting && collecting.id !== pot.id ? collecting.name : undefined }
            : undefined
        }
        onSubmit={save}
      />
      {/* Rule T2: the default pot is never archived. */}
      {!pot.is_default && <ArchiveSection ledgerId={ledgerId} pot={pot} />}
    </Screen>
  );
}

/**
 * How far the pot is on its way: the row is a progress bar against the
 * target, below it what this payday period asks for (rule T8).
 */
function GoalSummary({ pot, goal }: { pot: Pot; goal?: SavingsGoal }) {
  const target = pot.target_cents ?? 0;
  return (
    <Card className="overflow-hidden py-0">
      <ProgressRow progress={potProgress(pot)} label={pot.name} className="gap-1 p-5">
        <Text variant="overline">Goal</Text>
        <Text className="font-display text-2xl">
          {target > 0 ? `${formatCents(pot.balance_cents)} of ${formatCents(target)}` : formatCents(pot.balance_cents)}
        </Text>
        <Text variant="muted">{describeGoal(pot)}</Text>
        {goal?.reached && <Text className="font-semibold text-positive">Goal reached</Text>}
        {goal && !goal.reached && goal.rate_cents > 0 && <Text variant="muted">{`This period: ${describeRate(goal)}`}</Text>}
      </ProgressRow>
    </Card>
  );
}

function ArchiveSection({ ledgerId, pot }: { ledgerId: string; pot: Pot }) {
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function setArchived(archived: boolean) {
    setBusy(true);
    setError(undefined);
    try {
      // Only the archive flag changes; the goal stays as it is.
      await pots.update(ledgerId, pot.id, { archived }, today());
      toast.success(archived ? "Pot archived" : "Pot restored");
      router.back();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  if (pot.archived) {
    return (
      <View className="gap-2">
        <Text variant="muted">This pot is archived. It accepts no new bookings.</Text>
        <Button variant="outline" onPress={() => setArchived(false)} disabled={busy}>
          <Text>Restore pot</Text>
        </Button>
        <ErrorMessage message={error} />
      </View>
    );
  }
  // Archiving keeps the bookings and the balance, so the message says that.
  const message = `Archive “${pot.name}”? It accepts no new bookings, but its ${formatCents(pot.balance_cents)} still count in the total. You can restore it later.`;
  return (
    <View className="gap-2">
      {confirming ? (
        <ConfirmPanel
          message={message}
          confirmLabel="Archive"
          busy={busy}
          onConfirm={() => setArchived(true)}
          onCancel={() => setConfirming(false)}
        />
      ) : (
        <Button variant="outline" onPress={() => setConfirming(true)}>
          <Text className="text-destructive">Archive pot</Text>
        </Button>
      )}
      <ErrorMessage message={error} />
    </View>
  );
}
