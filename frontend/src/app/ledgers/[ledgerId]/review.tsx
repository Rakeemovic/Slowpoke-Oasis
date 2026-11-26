import { router, useLocalSearchParams } from "expo-router";
import { ChartColumn, NotebookPen, TriangleAlert } from "lucide-react-native";
import { useCallback, useState } from "react";
import { View } from "react-native";
import { toast } from "sonner-native";

import { ledgers } from "@/api/endpoints";
import type { WeeklyReview } from "@/api/types";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { ListRow, ListSection } from "@/components/list";
import { PurposeRow } from "@/components/purpose-row";
import { SafeToSpend } from "@/components/safe-to-spend";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { WantRow } from "@/components/want-row";
import { useLoader } from "@/hooks/use-loader";
import { overPlan } from "@/lib/budget";
import { formatDate, today } from "@/lib/dates";
import { errorMessage } from "@/lib/errors";
import { MAIN_HUE, NEUTRAL_HUE } from "@/lib/hues";
import { formatCents } from "@/lib/money";

type DueReview = Extract<WeeklyReview, { due: true }>;

/**
 * The weekly review (rules W1–W3) on one page: where the own money of the last
 * days went, which wants were spontaneous, and how the budget stands.
 */
export default function ReviewScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  const load = useCallback(() => ledgers.weeklyReview(ledgerId, today()), [ledgerId]);
  const { data: review, error, loading, reload } = useLoader(load);

  return (
    <Screen>
      {loading && !review && <Skeleton className="h-64 w-full rounded-2xl" />}
      <ErrorMessage message={error} onRetry={reload} />
      {review?.due === false && (
        <EmptyState
          icon={NotebookPen}
          title="Nothing to review"
          text="The next review opens on your review day, once you have booked something."
        />
      )}
      {review?.due && <ReviewContent ledgerId={ledgerId} review={review} onChanged={reload} />}
    </Screen>
  );
}

function ReviewContent({ ledgerId, review, onChanged }: { ledgerId: string; review: DueReview; onChanged: () => Promise<void> }) {
  return (
    <>
      <Text variant="muted" className="text-center text-sm">
        {`${formatDate(review.covered_from)} – ${formatDate(review.covered_until)}`}
      </Text>
      <View className="gap-3">
        <Text variant="overline" role="heading" aria-level={2} className="px-1">
          Your week
        </Text>
        {review.lines.map((line) => (
          <PurposeRow key={line.purpose} line={line} comparedWith="the days before" />
        ))}
      </View>
      <Wants ledgerId={ledgerId} review={review} onChanged={onChanged} />
      <BudgetStatus ledgerId={ledgerId} />
      <FinishButton ledgerId={ledgerId} />
    </>
  );
}

function Wants({ ledgerId, review, onChanged }: { ledgerId: string; review: DueReview; onChanged: () => Promise<void> }) {
  const categories = useLoader(useCallback(() => ledgers.categories(ledgerId), [ledgerId]));
  const [error, setError] = useState<string>();
  const [saving, setSaving] = useState(false);

  async function mark(id: string, spontaneous: boolean) {
    setSaving(true);
    setError(undefined);
    try {
      await ledgers.markSpontaneous(ledgerId, id, spontaneous);
      // The wants line above counts the spontaneous part, so everything loads again.
      await onChanged();
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setSaving(false);
    }
  }

  if (review.wants.length === 0) {
    return null;
  }
  return (
    <View className="gap-2">
      <ListSection title="Was it spontaneous?">
        {review.wants.map((tx) => (
          <WantRow
            key={tx.id}
            tx={tx}
            categories={categories.data ?? []}
            disabled={saving}
            onChange={(spontaneous) => mark(tx.id, spontaneous)}
          />
        ))}
      </ListSection>
      <ErrorMessage message={error ?? categories.error} />
    </View>
  );
}

/** The budget in short: what is safe to spend, and the categories over their plan. */
function BudgetStatus({ ledgerId }: { ledgerId: string }) {
  const { data, error, reload } = useLoader(useCallback(() => ledgers.budget(ledgerId, today()), [ledgerId]));
  const budgetHref = { pathname: "/ledgers/[ledgerId]/budget", params: { ledgerId } } as const;
  return (
    <View className="gap-3">
      <Text variant="overline" role="heading" aria-level={2} className="px-1">
        Budget
      </Text>
      <Card>
        <CardContent>
          <SafeToSpend ledgerId={ledgerId} />
        </CardContent>
      </Card>
      <ErrorMessage message={error} onRetry={reload} />
      <ListSection>
        {data &&
          overPlan(data).map((c) => (
            <ListRow
              key={c.category_id}
              icon={TriangleAlert}
              hue={NEUTRAL_HUE}
              title={c.name}
              subtitle={`${formatCents(-c.difference_cents)} over plan`}
            />
          ))}
        <ListRow icon={ChartColumn} hue={MAIN_HUE} title="Open budget" href={budgetHref} />
      </ListSection>
    </View>
  );
}

function FinishButton({ ledgerId }: { ledgerId: string }) {
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);

  async function finish() {
    setBusy(true);
    setError(undefined);
    try {
      await ledgers.completeWeeklyReview(ledgerId, today());
      toast.success("Weekly review done");
      router.back();
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }

  return (
    <View className="gap-2">
      <ErrorMessage message={error} />
      <Button onPress={finish} disabled={busy}>
        <Text>{busy ? "Saving…" : "Finish review"}</Text>
      </Button>
    </View>
  );
}
