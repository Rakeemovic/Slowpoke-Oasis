import { Link, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { ledgers, pots as potsApi } from "@/api/endpoints";
import type { Category, Pot, Transaction } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { HeaderActions } from "@/components/header-actions";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { LedgerBalance } from "@/components/ledger-balance";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { usePagedLoader } from "@/hooks/use-paged-loader";
import { formatCents, formatSignedCents } from "@/lib/money";
import { potSummary, transactionTitle } from "@/lib/transactions";
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

  // Category and pot names are needed to show the rows, so these lists load too.
  const categoryList = useLoader(useCallback(() => ledgers.categories(ledgerId), [ledgerId]));
  const potList = useLoader(useCallback(() => potsApi.list(ledgerId), [ledgerId]));
  const list = usePagedLoader(
    useCallback((limit: number, offset: number) => ledgers.transactions(ledgerId, { limit, offset }), [ledgerId]),
  );
  const { items: transactions, loading, error, reload } = list;
  const categories = categoryList.data;

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
      <View className="flex-row gap-2">
        <Link href={{ pathname: "/ledgers/[ledgerId]/pots", params: { ledgerId } }} asChild>
          <Button className="flex-1" variant="outline">
            <Text>Pots</Text>
          </Button>
        </Link>
        <PlanButton ledgerId={ledgerId} pots={potList.data} />
        <Link href={{ pathname: "/ledgers/[ledgerId]/categories", params: { ledgerId } }} asChild>
          <Button className="flex-1" variant="outline">
            <Text>Categories</Text>
          </Button>
        </Link>
      </View>
      <Link href={{ pathname: "/ledgers/[ledgerId]/deleted", params: { ledgerId, kind } }} asChild>
        <Button variant="outline">
          <Text>Deleted transactions</Text>
        </Button>
      </Link>
      {householdId && (
        <Link href={{ pathname: "/households/[householdId]/members", params: { householdId } }} asChild>
          <Button variant="outline">
            <Text>Members</Text>
          </Button>
        </Link>
      )}
      {loading && !transactions && <ActivityIndicator />}
      <ErrorMessage message={error ?? categoryList.error ?? potList.error} onRetry={reload} />
      {transactions?.length === 0 && <Text variant="muted">No transactions yet. Tap “Add transaction” to book the first one.</Text>}
      {transactions && transactions.length > 0 && (
        <Card className="py-2">
          <CardContent className="gap-0 px-0">
            {transactions.map((tx, index) => (
              <TransactionRow
                key={tx.id}
                tx={tx}
                categories={categories ?? []}
                pots={potList.data ?? []}
                first={index === 0}
                showAuthor={showAuthor}
                kind={kind}
              />
            ))}
          </CardContent>
        </Card>
      )}
      {list.hasMore && (
        <Button variant="outline" onPress={list.loadMore} disabled={list.loadingMore}>
          <Text>{list.loadingMore ? "Loading…" : "Load more"}</Text>
        </Button>
      )}
    </Screen>
  );
}

function TransactionRow({
  tx,
  categories,
  pots,
  first,
  showAuthor,
  kind,
}: {
  tx: Transaction;
  categories: Category[];
  pots: Pot[];
  first: boolean;
  showAuthor: boolean;
  kind?: string;
}) {
  // The API sends signed amounts: money coming in is positive, money going out
  // negative. A transfer between pots of the same ledger changes neither.
  const isTransfer = tx.type === "transfer";
  const isIncoming = !isTransfer && tx.amount_cents > 0;
  const title = transactionTitle(tx, categories);
  const details = [tx.booked_on, potSummary(tx, pots), showAuthor && (tx.username ?? "Former member"), tx.note].filter(Boolean).join(" · ");

  const row = (
    <View className={cn("flex-row items-center gap-3 px-4 py-3", !first && "border-t border-border")}>
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">
          {title}
          {tx.is_spontaneous && " ⚡"}
        </Text>
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

/** Opens the plan of the default pot, which holds the monthly budget. */
function PlanButton({ ledgerId, pots }: { ledgerId: string; pots?: Pot[] }) {
  const defaultPot = pots?.find((p) => p.is_default);
  if (!defaultPot) {
    return (
      <Button className="flex-1" variant="outline" disabled>
        <Text>Plan</Text>
      </Button>
    );
  }
  return (
    <Link
      href={{ pathname: "/ledgers/[ledgerId]/pots/[potId]/plan", params: { ledgerId, potId: defaultPot.id } }}
      asChild
    >
      <Button className="flex-1" variant="outline">
        <Text>Plan</Text>
      </Button>
    </Link>
  );
}
