import { useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { Category, DeletedTransaction } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { errorMessage } from "@/lib/errors";
import { formatCents, formatSignedCents } from "@/lib/money";
import { transactionTitle } from "@/lib/transactions";
import { cn } from "@/lib/utils";

export default function DeletedTransactionsScreen() {
  const { ledgerId, kind } = useLocalSearchParams<{ ledgerId: string; kind?: string }>();
  const showAuthor = kind !== "personal";

  const load = useCallback(
    () => Promise.all([ledgers.deletedTransactions(ledgerId), ledgers.categories(ledgerId)]),
    [ledgerId],
  );
  const { data, error, loading, reload } = useLoader(load);
  const [items, categories] = data ?? [];

  // Only one restore at a time, so the buttons cannot race each other.
  const [restoringId, setRestoringId] = useState<string>();
  const [restoreError, setRestoreError] = useState<string>();

  async function restore(id: string) {
    setRestoringId(id);
    setRestoreError(undefined);
    try {
      await ledgers.restoreTransaction(ledgerId, id);
      await reload();
    } catch (e) {
      // The server decides who may restore (e.g. not someone else's booking), so its answer is shown as is.
      setRestoreError(errorMessage(e));
    } finally {
      setRestoringId(undefined);
    }
  }

  return (
    <Screen>
      <Text variant="muted">Deleted transactions are removed for good 30 days after deletion.</Text>
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      <ErrorMessage message={restoreError} />
      {items?.length === 0 && <Text variant="muted">Nothing to restore.</Text>}
      {items && items.length > 0 && (
        <Card className="py-2">
          <CardContent className="gap-0 px-0">
            {items.map((tx, index) => (
              <DeletedRow
                key={tx.id}
                tx={tx}
                categories={categories ?? []}
                first={index === 0}
                showAuthor={showAuthor}
                busy={restoringId !== undefined}
                onRestore={() => restore(tx.id)}
              />
            ))}
          </CardContent>
        </Card>
      )}
    </Screen>
  );
}

function DeletedRow({
  tx,
  categories,
  first,
  showAuthor,
  busy,
  onRestore,
}: {
  tx: DeletedTransaction;
  categories: Category[];
  first: boolean;
  showAuthor: boolean;
  busy: boolean;
  onRestore: () => void;
}) {
  const isTransfer = tx.type === "transfer";
  const title = transactionTitle(tx, categories);
  const deletedBy = showAuthor ? ` by ${tx.deleted_by_username ?? "a former member"}` : "";
  const details = [tx.booked_on, tx.note, `Deleted${deletedBy} on ${tx.deleted_at.slice(0, 10)}`].filter(Boolean).join(" · ");

  return (
    <View className={cn("flex-row items-center gap-3 px-4 py-3", !first && "border-t border-border")}>
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{title}</Text>
        <Text variant="muted" numberOfLines={2}>
          {details}
        </Text>
        <Text>{isTransfer ? formatCents(Math.abs(tx.amount_cents)) : formatSignedCents(tx.amount_cents)}</Text>
      </View>
      <Button variant="outline" size="sm" onPress={onRestore} disabled={busy} aria-label={`Restore ${title}`}>
        <Text>Restore</Text>
      </Button>
    </View>
  );
}
