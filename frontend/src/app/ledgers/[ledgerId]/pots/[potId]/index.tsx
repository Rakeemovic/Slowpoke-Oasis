import { Link, router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { pots } from "@/api/endpoints";
import type { Pot, PotChange } from "@/api/types";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { PotForm, type PotFormValues } from "@/components/pot-form";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";
import { formatCents } from "@/lib/money";

export default function PotScreen() {
  const { ledgerId, potId } = useLocalSearchParams<{ ledgerId: string; potId: string }>();
  const { data, error, loading, reload } = useLoader(useCallback(() => pots.list(ledgerId), [ledgerId]));
  const pot = data?.find((p) => p.id === potId);

  if (!pot) {
    return (
      <Screen>
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error ?? (data ? "This pot does not exist any more." : undefined)} onRetry={reload} />
      </Screen>
    );
  }

  async function save(values: PotFormValues) {
    await pots.update(ledgerId, potId, { ...values, archived: pot!.archived });
    router.back();
  }

  return (
    <Screen>
      <PotForm
        initial={{ name: pot.name, target_cents: pot.target_cents ?? 0, target_date: pot.target_date ?? "" }}
        hideGoal={pot.is_default}
        onSubmit={save}
      />
      <Link href={{ pathname: "/ledgers/[ledgerId]/pots/[potId]/plan", params: { ledgerId, potId } }} asChild>
        <Button variant="outline">
          <Text>Plan</Text>
        </Button>
      </Link>
      {/* Rule T2: the default pot is never archived. */}
      {!pot.is_default && <ArchiveSection ledgerId={ledgerId} pot={pot} />}
    </Screen>
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
      const change: PotChange = {
        name: pot.name,
        target_cents: pot.target_cents ?? 0,
        target_date: pot.target_date ?? "",
        archived,
      };
      await pots.update(ledgerId, pot.id, change);
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
