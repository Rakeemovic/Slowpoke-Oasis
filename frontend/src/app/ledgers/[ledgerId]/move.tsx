import { router, useLocalSearchParams } from "expo-router";

import { ledgers } from "@/api/endpoints";
import type { NewPotTransfer } from "@/api/types";
import { MoveForm } from "@/components/move-form";
import { Screen } from "@/components/screen";

/**
 * Moves money between two pots of this ledger, e.g. from “available” into a
 * savings pot. With toPotId and amountCents it starts prefilled from the
 * default pot, e.g. for the rate of a reserve.
 */
export default function MoveScreen() {
  const { ledgerId, toPotId, amountCents } = useLocalSearchParams<{
    ledgerId: string;
    toPotId?: string;
    amountCents?: string;
  }>();
  const initial = toPotId ? { counterpart_pot_id: toPotId, amount_cents: Number(amountCents) || undefined } : undefined;

  async function create(values: NewPotTransfer) {
    await ledgers.createTransaction(ledgerId, { ...values, note: values.note || undefined });
    router.back();
  }

  return (
    <Screen>
      <MoveForm ledgerId={ledgerId} initial={initial} onSubmit={create} />
    </Screen>
  );
}
