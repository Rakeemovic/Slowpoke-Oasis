import { Link, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { Category, Transaction } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { HeaderActions } from "@/components/header-actions";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { LedgerBalance } from "@/components/ledger-balance";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatCents, formatSignedCents } from "@/lib/money";
import { transactionTitle } from "@/lib/transactions";
import { cn } from "@/lib/utils";

export default function TransactionsScreen() {
  const { ledgerId, name, kind, householdId } = useLocalSearchParams<{
    ledgerId: string;
    name?: string;
    kind?: string;
    householdId?: string;
  }>();
  // In a personal ledger every booking is the user's own, so naming them is noise.
  const showAuthor = kind !== "personal";

  // Both lists are needed to show category names, so they load together.
  const load = useCallback(
    () => Promise.all([ledgers.transactions(ledgerId), ledgers.categories(ledgerId)]),
    [ledgerId],
  );
  const { data, error, loading, reload } = useLoader(load);
  const [transactions, categories] = data ?? [];

  return (
    <Screen>
      <Stack.Screen options={{ title: name ?? "Transactions", headerRight: () => <HeaderActions /> }} />
      <Card>
        <CardContent>
          <LedgerBalance ledgerId={ledgerId} size="large" />
        </CardContent>
      </Card>
      <View className="flex-row gap-2">
        <Link href={{ pathname: "/ledgers/[ledgerId]/new", params: { ledgerId } }} asChild>
          <Button className="flex-1">
            <Text>Add transaction</Text>
          </Button>
        </Link>
        <Link href={{ pathname: "/ledgers/[ledgerId]/transfer", params: { ledgerId } }} asChild>
          <Button className="flex-1" variant="outline">
            <Text>Transfer</Text>
          </Button>
        </Link>
      </View>
      {householdId && (
        <Link href={{ pathname: "/households/[householdId]/members", params: { householdId } }} asChild>
          <Button variant="outline">
            <Text>Members</Text>
          </Button>
        </Link>
      )}
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      {transactions?.length === 0 && <Text variant="muted">No transactions yet. Tap “Add transaction” to book the first one.</Text>}
      {transactions && transactions.length > 0 && (
        <Card className="py-2">
          <CardContent className="gap-0 px-0">
            {transactions.map((tx, index) => (
              <TransactionRow
                key={tx.id}
                tx={tx}
                categories={categories ?? []}
                first={index === 0}
                showAuthor={showAuthor}
                kind={kind}
              />
            ))}
          </CardContent>
        </Card>
      )}
    </Screen>
  );
}

function TransactionRow({
  tx,
  categories,
  first,
  showAuthor,
  kind,
}: {
  tx: Transaction;
  categories: Category[];
  first: boolean;
  showAuthor: boolean;
  kind?: string;
}) {
  // The API sends signed amounts: money coming in is positive, money going out
  // negative. A transfer between pots of the same ledger changes neither.
  const isTransfer = tx.type === "transfer";
  const isIncoming = !isTransfer && tx.amount_cents > 0;
  const title = transactionTitle(tx, categories);
  const details = [tx.booked_on, showAuthor && (tx.username ?? "Former member"), tx.note].filter(Boolean).join(" · ");

  const row = (
    <View className={cn("flex-row items-center gap-3 px-4 py-3", !first && "border-t border-border")}>
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{title}</Text>
        <Text variant="muted" numberOfLines={1}>
          {details}
        </Text>
      </View>
      <Text className={cn("font-semibold", isIncoming && "text-green-600 dark:text-green-500")}>
        {isTransfer ? formatCents(Math.abs(tx.amount_cents)) : formatSignedCents(tx.amount_cents)}
      </Text>
    </View>
  );
  // Rows the user cannot change open nothing, so they are not pressable.
  if (!tx.can_edit && !tx.can_delete) {
    return row;
  }
  return (
    <Link
      href={{ pathname: "/ledgers/[ledgerId]/transactions/[transactionId]", params: { ledgerId: tx.ledger_id, transactionId: tx.id, kind } }}
      asChild>
      <Pressable className="active:bg-accent web:hover:bg-accent/50" role="link">
        {row}
      </Pressable>
    </Link>
  );
}
