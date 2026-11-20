import { router, useLocalSearchParams } from "expo-router";

import { ledgers } from "@/api/endpoints";
import type { NewPotTransfer } from "@/api/types";
import { MoveForm } from "@/components/move-form";
import { Screen } from "@/components/screen";

/** Moves money between two pots of this ledger, e.g. from “available” into a savings pot. */
export default function MoveScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();

  async function create(values: NewPotTransfer) {
    await ledgers.createTransaction(ledgerId, { ...values, note: values.note || undefined });
    router.back();
  }

  return (
    <Screen>
      <MoveForm ledgerId={ledgerId} onSubmit={create} />
    </Screen>
  );
}
