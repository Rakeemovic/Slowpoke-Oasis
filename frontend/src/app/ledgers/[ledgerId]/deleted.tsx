import { useLocalSearchParams } from "expo-router";
import { Trash2 } from "lucide-react-native";
import { useCallback, useState } from "react";
import { View } from "react-native";
import { toast } from "sonner-native";

import { ledgers } from "@/api/endpoints";
import type { Category, DeletedTransaction } from "@/api/types";
import { Amount } from "@/components/amount";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { IconTile } from "@/components/icon-tile";
import { ListSection } from "@/components/list";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { usePagedLoader } from "@/hooks/use-paged-loader";
import { errorMessage } from "@/lib/errors";
import { hueFor, NEUTRAL_HUE } from "@/lib/hues";
import { categoryIcon, TRANSFER_ICON } from "@/lib/icons";
import { transactionTitle } from "@/lib/transactions";

export default function DeletedTransactionsScreen() {
  const { ledgerId, kind } = useLocalSearchParams<{ ledgerId: string; kind?: string }>();
  const showAuthor = kind !== "personal";

  // Category names are needed to show the rows; the deleted entries come in pages.
  const categoryList = useLoader(useCallback(() => ledgers.categories(ledgerId), [ledgerId]));
  const list = usePagedLoader(
    useCallback((limit: number, offset: number) => ledgers.deletedTransactions(ledgerId, { limit, offset }), [ledgerId]),
  );
  const { items, error, loading, reload } = list;
  const categories = categoryList.data;

  // Only one restore at a time, so the buttons cannot race each other.
  const [restoringId, setRestoringId] = useState<string>();
  const [restoreError, setRestoreError] = useState<string>();

  async function restore(id: string) {
    setRestoringId(id);
    setRestoreError(undefined);
    try {
      await ledgers.restoreTransaction(ledgerId, id);
      toast.success("Transaction restored");
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
      {loading && !items && <Skeleton className="h-40 w-full rounded-2xl" />}
      <ErrorMessage message={error ?? categoryList.error} onRetry={reload} />
      <ErrorMessage message={restoreError} />
      {items?.length === 0 && <EmptyState icon={Trash2} title="Nothing to restore" />}
      {items && items.length > 0 && (
        <ListSection>
          {items.map((tx) => (
            <DeletedRow
              key={tx.id}
              tx={tx}
              categories={categories ?? []}
              showAuthor={showAuthor}
              busy={restoringId !== undefined}
              onRestore={() => restore(tx.id)}
            />
          ))}
        </ListSection>
      )}
      {list.hasMore && (
        <Button variant="outline" onPress={list.loadMore} disabled={list.loadingMore}>
          <Text>{list.loadingMore ? "Loading…" : "Load more"}</Text>
        </Button>
      )}
    </Screen>
  );
}

function DeletedRow({
  tx,
  categories,
  showAuthor,
  busy,
  onRestore,
}: {
  tx: DeletedTransaction;
  categories: Category[];
  showAuthor: boolean;
  busy: boolean;
  onRestore: () => void;
}) {
  const isTransfer = tx.type === "transfer";
  const title = transactionTitle(tx, categories);
  const deletedBy = showAuthor ? ` by ${tx.deleted_by_username ?? "a former member"}` : "";
  const details = [tx.booked_on, tx.note, `Deleted${deletedBy} on ${tx.deleted_at.slice(0, 10)}`].filter(Boolean).join(" · ");
  const category = categories.find((c) => c.id === tx.category_id);

  return (
    <View className="flex-row items-center gap-3 px-4 py-3">
      <IconTile
        icon={category ? categoryIcon(category.name) : TRANSFER_ICON}
        hue={category ? hueFor(category.id) : NEUTRAL_HUE}
      />
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{title}</Text>
        <Text variant="muted" numberOfLines={2}>
          {details}
        </Text>
        <Amount cents={isTransfer ? Math.abs(tx.amount_cents) : tx.amount_cents} signed={!isTransfer} size="sm" />
      </View>
      <Button variant="outline" size="sm" onPress={onRestore} disabled={busy} aria-label={`Restore ${title}`}>
        <Text>Restore</Text>
      </Button>
    </View>
  );
}
