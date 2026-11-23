import { Link, Stack, useLocalSearchParams } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";

import { ledgers, recurring } from "@/api/endpoints";
import type { Category, RecurringPayment } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { HeaderActions } from "@/components/header-actions";
import { Screen } from "@/components/screen";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";
import { useLoader } from "@/hooks/use-loader";
import { formatDate, today } from "@/lib/dates";
import { loadMyLedgers } from "@/lib/ledgers";
import { formatCents } from "@/lib/money";
import { describeSchedule } from "@/lib/recurring";
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
    <Screen>
      <Stack.Screen options={{ title: "Recurring payments", headerRight: () => <HeaderActions /> }} />
      <Link href={{ pathname: "/ledgers/[ledgerId]/recurring/new", params: { ledgerId } }} asChild>
        <Button>
          <Text>New recurring payment</Text>
        </Button>
      </Link>
      <Text variant="muted">
        Recurring payments are reminders: they show up under “Due payments” in the ledger, and nothing is booked until
        you tap “Book”.
      </Text>
      {data && !isOwner && <Text variant="muted">Only the owner of the household can change or archive them.</Text>}
      {loading && !data && <ActivityIndicator />}
      <ErrorMessage message={error} onRetry={reload} />
      {data && payments.length === 0 && <Text variant="muted">No recurring payments yet.</Text>}
      {groups.map(
        (g) =>
          g.items.length > 0 && (
            <View key={g.title} className="gap-2">
              <Text variant="muted">{g.title}</Text>
              <PaymentList ledgerId={ledgerId} payments={g.items} categories={categories} linked={isOwner} />
            </View>
          ),
      )}
    </Screen>
  );
}

function PaymentList({
  ledgerId,
  payments,
  categories,
  linked,
}: {
  ledgerId: string;
  payments: RecurringPayment[];
  categories: Category[];
  linked: boolean;
}) {
  return (
    <Card className="py-2">
      <CardContent className="gap-0 px-0">
        {payments.map((p, index) => {
          const row = <PaymentRow payment={p} categories={categories} first={index === 0} />;
          if (!linked) {
            return <View key={p.id}>{row}</View>;
          }
          return (
            <Link
              key={p.id}
              href={{ pathname: "/ledgers/[ledgerId]/recurring/[paymentId]", params: { ledgerId, paymentId: p.id } }}
              asChild>
              <Pressable className="active:bg-accent web:hover:bg-accent/50" role="link">
                {row}
              </Pressable>
            </Link>
          );
        })}
      </CardContent>
    </Card>
  );
}

function PaymentRow({ payment: p, categories, first }: { payment: RecurringPayment; categories: Category[]; first: boolean }) {
  const category = categories.find((c) => c.id === p.category_id)?.name;
  const next = !p.archived && (p.next_due_on ? `next ${formatDate(p.next_due_on)}` : "ended");
  const details = [describeSchedule(p), category, next].filter(Boolean).join(" · ");
  return (
    <View className={cn("flex-row items-center gap-3 px-4 py-3", !first && "border-t border-border")}>
      <View className="flex-1 gap-0.5">
        <Text className="font-medium">{p.name}</Text>
        <Text variant="muted">{details}</Text>
      </View>
      <Text className="font-semibold">{formatCents(p.amount_cents)}</Text>
    </View>
  );
}
