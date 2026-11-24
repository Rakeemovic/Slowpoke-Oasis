import { router, useLocalSearchParams } from "expo-router";
import { CircleCheck, TriangleAlert } from "lucide-react-native";
import { useCallback, useState } from "react";
import { View } from "react-native";
import { toast } from "sonner-native";

import { ledgers } from "@/api/endpoints";
import type { Balance, Reconciliation } from "@/api/types";
import { Amount } from "@/components/amount";
import { ErrorMessage } from "@/components/error-message";
import { FormField } from "@/components/form-field";
import { KindButton, Segmented } from "@/components/kind-button";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Icon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { formatCents, formatSignedCents, parseBalanceToCents } from "@/lib/money";
import { cn } from "@/lib/utils";

/** An account that can be compared with the bank: the main account or a real sub-account (rule L9). */
interface Account {
  /** Undefined for the bank account, which the API addresses without pot_id. */
  potId?: string;
  name: string;
  /** What the app expects the bank to show. */
  expectedCents: number;
  reconciled: Reconciliation | null;
}

function accountsOf(balance: Balance): Account[] {
  return [
    { name: "Bank account", expectedCents: balance.estimated_cents, reconciled: balance.reconciled },
    ...balance.separate_accounts.map((a) => ({
      potId: a.pot_id,
      name: a.name,
      expectedCents: a.balance_cents,
      reconciled: a.reconciled,
    })),
  ];
}

/**
 * Reconciliation: the user types the balance their bank shows, the app shows
 * the difference to what it expects and can book it as a correction. Only the
 * owner gets here (the entry points are hidden for members; the server checks
 * again).
 */
export default function ReconcileScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  const { data, error, reload } = useLoader(useCallback(() => ledgers.balance(ledgerId, today()), [ledgerId]));
  const [selected, setSelected] = useState(0);

  if (!data) {
    return (
      <Screen>
        <ErrorMessage message={error} onRetry={reload} />
        {!error && <Skeleton className="h-48 w-full rounded-2xl" />}
      </Screen>
    );
  }
  const accounts = accountsOf(data);
  const account = accounts[Math.min(selected, accounts.length - 1)];
  return (
    <Screen>
      <Text variant="muted">
        Compare the app with your bank. Type the balance your banking app shows today; any difference can be booked as a
        correction, so both agree again.
      </Text>
      {accounts.length > 1 && (
        <Segmented>
          {accounts.map((a, index) => (
            <KindButton key={a.potId ?? "bank"} label={a.name} active={index === selected} onPress={() => setSelected(index)} />
          ))}
        </Segmented>
      )}
      {/* A new key resets the form when another account is chosen. */}
      <ReconcileForm key={account.potId ?? "bank"} ledgerId={ledgerId} account={account} balance={data} />
    </Screen>
  );
}

function ReconcileForm({ ledgerId, account, balance }: { ledgerId: string; account: Account; balance: Balance }) {
  const [input, setInput] = useState("");
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const actual = parseBalanceToCents(input);
  const difference = actual === null ? null : actual - account.expectedCents;
  const isBank = account.potId === undefined;

  async function save(correct: boolean) {
    if (actual === null) {
      setError("Enter the balance as an amount, e.g. 1234,56 or -20.");
      return;
    }
    setBusy(true);
    setError(undefined);
    try {
      const result = await ledgers.reconcile(
        ledgerId,
        { actual_cents: actual, correct, ...(account.potId && { pot_id: account.potId }) },
        today(),
      );
      toast.success(
        result.correction_id ? `Correction of ${formatSignedCents(result.difference_cents)} booked` : "Reconciliation saved",
      );
      router.back();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <View className="gap-6">
      <Card>
        <CardContent className="gap-4">
          <View className="flex-row items-center justify-between gap-3">
            <Text variant="muted">{isBank ? "Estimated account balance" : `${account.name} in the app`}</Text>
            <Amount cents={account.expectedCents} size="md" aria-label={`Expected: ${formatCents(account.expectedCents)}`} />
          </View>
          <FormField
            label="Balance at your bank (€)"
            value={input}
            onChangeText={setInput}
            keyboardType="numbers-and-punctuation"
            placeholder="0,00"
            autoFocus
          />
          {difference !== null && <Difference cents={difference} />}
          {account.reconciled && (
            <Text variant="muted" className="text-xs">
              {`Last reconciled on ${formatDate(account.reconciled.reconciled_on)} at ${formatCents(account.reconciled.actual_cents)}`}
            </Text>
          )}
        </CardContent>
      </Card>
      {isBank && balance.unbooked_count > 0 && (
        <View className="flex-row gap-3 rounded-2xl bg-warning/10 p-4">
          <Icon as={TriangleAlert} size={18} className="text-warning" />
          <Text className="flex-1 text-sm">
            {`The estimate already subtracts ${balance.unbooked_count} unbooked ${balance.unbooked_count === 1 ? "payment" : "payments"} (${formatCents(-balance.unbooked_cents)}). If your bank has not paid ${balance.unbooked_count === 1 ? "it" : "them"} yet, wait or book ${balance.unbooked_count === 1 ? "it" : "them"} first instead of correcting.`}
          </Text>
        </View>
      )}
      <ErrorMessage message={error} />
      <View className="gap-2">
        {difference !== 0 && (
          <Button onPress={() => save(true)} disabled={busy || difference === null}>
            <Text>Book correction and save</Text>
          </Button>
        )}
        <Button
          variant={difference === 0 ? "default" : "outline"}
          onPress={() => save(false)}
          disabled={busy || difference === null}>
          <Text>{difference === 0 ? "Save" : "Save without correction"}</Text>
        </Button>
      </View>
    </View>
  );
}

/** The gap between bank and app, with a clear verdict instead of a bare number. */
function Difference({ cents }: { cents: number }) {
  const matches = cents === 0;
  return (
    <View
      aria-label={matches ? "Difference: none" : `Difference: ${formatSignedCents(cents)}`}
      className={cn("flex-row items-center gap-3 rounded-xl p-3", matches ? "bg-positive/10" : "bg-warning/10")}>
      <Icon as={matches ? CircleCheck : TriangleAlert} size={20} className={matches ? "text-positive" : "text-warning"} />
      <View className="flex-1">
        <Text className="font-medium">{matches ? "Bank and app agree" : `Difference: ${formatSignedCents(cents)}`}</Text>
        {!matches && (
          <Text variant="muted" className="text-xs">
            {cents > 0 ? "Your bank shows more than the app." : "Your bank shows less than the app."}
          </Text>
        )}
      </View>
    </View>
  );
}
