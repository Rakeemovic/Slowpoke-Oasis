import { router, useLocalSearchParams } from "expo-router";
import { toast } from "sonner-native";

import { pots } from "@/api/endpoints";
import { PotForm, type PotFormValues } from "@/components/pot-form";
import { Screen } from "@/components/screen";

export default function NewPotScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();

  async function create(values: PotFormValues) {
    await pots.create(ledgerId, {
      name: values.name,
      target_cents: values.target_cents || undefined,
      target_date: values.target_date || undefined,
      monthly_rate_cents: values.monthly_rate_cents || undefined,
      is_separate_account: values.is_separate_account || undefined,
    });
    toast.success("Pot created");
    router.back();
  }

  return (
    <Screen>
      <PotForm onSubmit={create} />
    </Screen>
  );
}
