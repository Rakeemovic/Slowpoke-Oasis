import { router, useLocalSearchParams } from "expo-router";
import { toast } from "sonner-native";

import { recurring } from "@/api/endpoints";
import type { RecurringPaymentInput } from "@/api/types";
import { RecurringForm } from "@/components/recurring-form";
import { Screen } from "@/components/screen";
import { today } from "@/lib/dates";

export default function NewRecurringPaymentScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();

  async function create(values: RecurringPaymentInput) {
    await recurring.create(ledgerId, values, today());
    toast.success("Recurring payment added");
    router.back();
  }

  return (
    <Screen>
      <RecurringForm ledgerId={ledgerId} onSubmit={create} />
    </Screen>
  );
}
