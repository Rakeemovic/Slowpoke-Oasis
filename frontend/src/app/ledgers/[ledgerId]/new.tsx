import { router, useLocalSearchParams } from "expo-router";

import { ledgers } from "@/api/endpoints";
import type { TransactionChange } from "@/api/types";
import { BookingForm } from "@/components/booking-form";
import { Screen } from "@/components/screen";

export default function NewTransactionScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();

  async function create(values: TransactionChange) {
    await ledgers.createTransaction(ledgerId, { ...values, note: values.note || undefined });
    router.back();
  }

  return (
    <Screen>
      <BookingForm ledgerId={ledgerId} onSubmit={create} />
    </Screen>
  );
}
