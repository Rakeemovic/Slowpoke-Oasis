import { router, useLocalSearchParams } from "expo-router";
import { useCallback, useState } from "react";
import { ActivityIndicator, View } from "react-native";

import { ledgers } from "@/api/endpoints";
import type { Category } from "@/api/types";
import { CategoryForm, type CategoryFormValues } from "@/components/category-form";
import { ConfirmPanel } from "@/components/confirm-panel";
import { ErrorMessage } from "@/components/error-message";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { bookingCount, fallbackName } from "@/lib/categories";
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
          <DeleteSection ledgerId={ledgerId} category={category} target={fallbackName(data, category.is_income)} />
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

function DeleteSection({ ledgerId, category, target }: { ledgerId: string; category: Category; target: string }) {
  const [confirming, setConfirming] = useState(false);
  const { busy, error, run } = useLeavingAction();

  // Name every consequence before an action that cannot be undone (rule K2).
  const moved =
    category.transaction_count > 0
      ? ` Its ${bookingCount(category.transaction_count)} and planned amounts move to “${target}”.`
      : ` Planned amounts move to “${target}”.`;
  const message = `Delete “${category.name}”?${moved} This cannot be undone. To keep the history apart, archive it instead.`;

  return (
    <View className="gap-2">
      {confirming ? (
        <ConfirmPanel
          message={message}
          confirmLabel="Delete"
          busy={busy}
          onConfirm={() => run(() => ledgers.deleteCategory(ledgerId, category.id))}
          onCancel={() => setConfirming(false)}
        />
      ) : (
        <Button variant="outline" onPress={() => setConfirming(true)}>
          <Text className="text-destructive">Delete category</Text>
        </Button>
      )}
      <ErrorMessage message={error} />
    </View>
  );
}
