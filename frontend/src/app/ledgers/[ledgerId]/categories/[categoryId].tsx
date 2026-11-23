import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { Category } from "@/api/types";
import { CategoryForm, type CategoryFormValues } from "@/components/category-form";
import { ChoicePicker } from "@/components/choice-picker";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { bookingCount, fallbackName, moveTargets } from "@/lib/categories";
import { errorMessage } from "@/lib/errors";

/** Owner only (rule K9): rename a category, change its purpose, archive or delete it. */
export default function CategoryScreen() {
  const { ledgerId, categoryId } = useLocalSearchParams<{ ledgerId: string; categoryId: string }>();
  const { data, error, loading, reload } = useLoader(useCallback(() => ledgers.categories(ledgerId), [ledgerId]));
  const category = data?.find((c) => c.id === categoryId);

  if (!data || !category) {
    return (
      <Screen>
        {loading && <ActivityIndicator />}
        <ErrorMessage message={error ?? (data ? "This category does not exist any more." : undefined)} onRetry={reload} />
      </Screen>
    );
  }

  async function save(values: CategoryFormValues) {
    await ledgers.updateCategory(ledgerId, categoryId, { name: values.name, purpose: values.purpose });
    router.back();
  }

  return (
    <Screen>
      <CategoryForm
        initial={{ name: category.name, is_income: category.is_income, purpose: category.purpose ?? "" }}
        onSubmit={save}
      />
      {/* Rule K6: the catch-all categories stay, so they can always take over bookings. */}
      {category.is_fallback ? (
        <Text variant="muted">
          This is the catch-all category: bookings of deleted categories move here. It can be renamed, but not archived or
          deleted.
        </Text>
      ) : (
        <>
          <ArchiveSection ledgerId={ledgerId} category={category} />
          <DeleteSection ledgerId={ledgerId} category={category} categories={data} />
        </>
      )}
    </Screen>
  );
}

/** Runs an action that ends on this screen: on success it goes back, on failure it shows the error. */
function useLeavingAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError(undefined);
    try {
      await action();
      router.back();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }
  return { busy, error, run };
}

function ArchiveSection({ ledgerId, category }: { ledgerId: string; category: Category }) {
  const { busy, error, run } = useLeavingAction();
  const setArchived = (archived: boolean) => run(() => ledgers.updateCategory(ledgerId, category.id, { archived }));

  return (
    <View className="gap-2">
      {category.archived ? (
        <>
          <Text variant="muted">This category is archived. It accepts no new bookings.</Text>
          <Button variant="outline" onPress={() => setArchived(false)} disabled={busy}>
            <Text>Restore category</Text>
          </Button>
        </>
      ) : (
        <Button variant="outline" onPress={() => setArchived(true)} disabled={busy}>
          <Text>Archive category</Text>
        </Button>
      )}
      <ErrorMessage message={error} />
    </View>
  );
}

/** Stands for the fallback category in the picker while the ledger has none yet; the server creates it. */
const NEW_FALLBACK = "new-fallback";

function DeleteSection({ ledgerId, category, categories }: { ledgerId: string; category: Category; categories: Category[] }) {
  const [confirming, setConfirming] = useState(false);
  const { busy, error, run } = useLeavingAction();

  // The owner may pick where the bookings go; the fallback is the default (rules K2, K6).
  const targets = moveTargets(categories, category).map((c) => ({ id: c.id, label: c.name }));
  const fallback = categories.find((c) => c.is_fallback && c.is_income === category.is_income);
  if (!fallback) {
    targets.unshift({ id: NEW_FALLBACK, label: fallbackName(categories, category.is_income) });
  }
  const [targetId, setTargetId] = useState(fallback?.id ?? NEW_FALLBACK);
  const target = targets.find((t) => t.id === targetId)?.label ?? "";

  // Name every consequence before an action that cannot be undone (rule K2).
  const moved =
    category.transaction_count > 0
      ? ` Its ${bookingCount(category.transaction_count)} and planned amounts move to “${target}”.`
      : ` Planned amounts move to “${target}”.`;
  const message = `Delete “${category.name}”?${moved} This cannot be undone. To keep the history apart, archive it instead.`;
  const remove = () => run(() => ledgers.deleteCategory(ledgerId, category.id, targetId === NEW_FALLBACK ? undefined : targetId));

  return (
    <View className="gap-2">
      {confirming ? (
        <>
          <Label>Move bookings to</Label>
          <ChoicePicker choices={targets} selected={targetId} onSelect={setTargetId} emptyText="" />
          <ConfirmPanel
            message={message}
            confirmLabel="Delete"
            busy={busy}
            onConfirm={remove}
            onCancel={() => setConfirming(false)}
          />
        </>
      ) : (
        <Button variant="outline" onPress={() => setConfirming(true)}>
          <Text className="text-destructive">Delete category</Text>
        </Button>
      )}
      <ErrorMessage message={error} />
    </View>
  );
}
