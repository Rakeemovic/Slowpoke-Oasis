import { router, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator } from "react-native";
import { toast } from "sonner-native";

import { auth, pots } from "@/api/endpoints";
import { ErrorMessage } from "@/components/error-message";
import { PotForm, type PotFormValues } from "@/components/pot-form";
import { Screen } from "@/components/screen";
import { useLoader } from "@/hooks/use-loader";
import { today } from "@/lib/dates";
import { collectingPot } from "@/lib/round-ups";

export default function NewPotScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  // Round-ups exist only in the personal ledger (rule T10); the form names the
  // pot that collects them now, because the new pot would take over.
  const { data, error, loading, reload } = useLoader(
    useCallback(async () => {
      const [list, me] = await Promise.all([pots.list(ledgerId), auth.me()]);
      return { collecting: collectingPot(list), isPersonal: me.personal_ledger_id === ledgerId };
    }, [ledgerId]),
  );

  async function create(values: PotFormValues) {
    await pots.create(
      ledgerId,
      {
        name: values.name,
        target_cents: values.target_cents || undefined,
        target_date: values.target_date || undefined,
        monthly_rate_cents: values.monthly_rate_cents || undefined,
        is_separate_account: values.is_separate_account || undefined,
        round_up_multiplier: values.round_up_multiplier || undefined,
      },
      today(),
    );
    toast.success("Pot created");
    router.back();
  }

  if (!data) {
    return (
      <Screen>
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error} onRetry={reload} />
      </Screen>
    );
  }

  return (
    <Screen>
      <PotForm onSubmit={create} roundUp={data.isPersonal ? { replaces: data.collecting?.name } : undefined} />
    </Screen>
  );
}
