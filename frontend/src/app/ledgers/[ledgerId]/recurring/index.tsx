import { Stack, useLocalSearchParams } from "expo-router";
import { Repeat } from "lucide-react-native";
import { useCallback } from "react";

import { ledgers, recurring } from "@/api/endpoints";
import type { Category, RecurringPayment } from "@/api/types";
import { Amount } from "@/components/amount";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { FloatingAction } from "@/components/floating-action";
import { ListRow, ListSection } from "@/components/list";
import { HeaderActions } from "@/components/header-actions";
import { Screen } from "@/components/screen";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, today } from "@/lib/dates";
import { loadMyLedgers } from "@/lib/ledgers";
import { describeSchedule } from "@/lib/recurring";
import { hueFor } from "@/lib/hues";
import { categoryIcon } from "@/lib/icons";
import { cn } from "@/lib/utils";

/**
 * All recurring payments of a ledger. Every member may add one; only the owner
 * may open one to change or archive it (rule R8), so for others the rows are
 * plain text.
 */
export default function RecurringPaymentsScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();
  const load = useCallback(
    () => Promise.all([recurring.list(ledgerId, today()), ledgers.categories(ledgerId), loadMyLedgers()]),
    [ledgerId],
  );
  const { data, error, loading, reload } = useLoader(load);
  const [payments, categories, myLedgers] = data ?? [[], [], []];
  const isOwner = myLedgers.find((l) => l.id === ledgerId)?.isOwner ?? false;

  const groups = [
    { title: "Active", items: payments.filter((p) => !p.archived) },
    { title: "Archived", items: payments.filter((p) => p.archived) },
  ];

  return (
    <Screen
      floating={
        <FloatingAction
          label="New recurring payment"
          href={{ pathname: "/ledgers/[ledgerId]/recurring/new", params: { ledgerId } }}
        />
      }>
      <Stack.Screen options={{ title: "Recurring payments", headerRight: () => <HeaderActions /> }} />
      <Text variant="muted">
        Recurring payments are reminders: they show up under “Due payments” in the ledger, and nothing is booked until you tap
        “Book”.
      </Text>
      {data && !isOwner && <Text variant="muted">Only the owner of the household can change or archive them.</Text>}
      {loading && !data && <Skeleton className="h-40 w-full rounded-2xl" />}
      <ErrorMessage message={error} onRetry={reload} />
      {data && payments.length === 0 && (
        <EmptyState
          icon={Repeat}
          title="No recurring payments yet"
          text="Add rent, subscriptions or insurance once, and the app reminds you when they are due."
        />
      )}
      {groups.map(
        (g) =>
          g.items.length > 0 && (
            <ListSection key={g.title} title={g.title}>
              {g.items.map((p) => (
                <ListRow
                  key={p.id}
                  icon={categoryIcon(categories.find((c) => c.id === p.category_id)?.name)}
                  hue={hueFor(p.category_id)}
                  title={p.name}
                  subtitle={details(p, categories)}
                  trailing={<Amount cents={p.amount_cents} />}
                  className={cn(p.archived && "opacity-60")}
                  href={
                    isOwner
                      ? { pathname: "/ledgers/[ledgerId]/recurring/[paymentId]", params: { ledgerId, paymentId: p.id } }
                      : undefined
                  }
                />
              ))}
            </ListSection>
          ),
      )}
    </Screen>
  );
}

function details(p: RecurringPayment, categories: Category[]): string {
  const category = categories.find((c) => c.id === p.category_id)?.name;
  const next = !p.archived && (p.next_due_on ? `next ${formatDate(p.next_due_on)}` : "ended");
  return [describeSchedule(p), category, next].filter(Boolean).join(" · ");
}
