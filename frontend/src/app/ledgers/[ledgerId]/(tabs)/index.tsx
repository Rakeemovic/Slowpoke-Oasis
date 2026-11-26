import { Link } from "expo-router";
import { ArrowLeftRight, ArrowUpDown, ReceiptText, Scale, type LucideIcon } from "lucide-react-native";
import { useCallback, useState } from "react";
import { Pressable, View } from "react-native";

import { ledgers, pots as potsApi } from "@/api/endpoints";
import { DuePayments } from "@/components/due-payments";
import { EmptyState } from "@/components/empty-state";
import { ErrorMessage } from "@/components/error-message";
import { FloatingAction } from "@/components/floating-action";
import { LedgerBalance } from "@/components/ledger-balance";
import { ListSection } from "@/components/list";
import { BlurFade } from "@/components/magic/blur-fade";
import { SafeToSpend } from "@/components/safe-to-spend";
import { Screen } from "@/components/screen";
import { TransactionRow } from "@/components/transaction-row";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Icon } from "@/components/ui/icon";
import { Skeleton } from "@/components/ui/skeleton";
import { Text } from "@/components/ui/text";
import { WeeklyReviewCard } from "@/components/weekly-review-card";
import { WrapUpCard } from "@/components/wrap-up-card";
import { useLoader } from "@/hooks/use-loader";
import { usePagedLoader } from "@/hooks/use-paged-loader";
import { dayHeading } from "@/lib/dates";
import { useLedger } from "@/lib/ledger-context";
import { groupByDay } from "@/lib/transactions";

/**
 * The home of a ledger, ordered by what matters most: how much there is and
 * how much may be spent, what is due, then what happened. The main action,
 * booking, floats at the bottom where the thumb is.
 */
export default function LedgerOverviewScreen() {
  const { ledgerId, kind, isOwner } = useLedger();
  // In a personal ledger every booking is the user's own, so naming them is noise.
  const showAuthor = kind !== "personal";

  // Category and pot names are needed to show the rows, so these lists load too.
  const categoryList = useLoader(useCallback(() => ledgers.categories(ledgerId), [ledgerId]));
  const potList = useLoader(useCallback(() => potsApi.list(ledgerId), [ledgerId]));
  const list = usePagedLoader(
    useCallback((limit: number, offset: number) => ledgers.transactions(ledgerId, { limit, offset }), [ledgerId]),
  );
  const { items: transactions, loading, error, reload } = list;
  // Bumped when a due payment is skipped, so the estimated balance and safe to spend reload.
  const [balanceVersion, setBalanceVersion] = useState(0);

  return (
    <Screen
      floating={<FloatingAction label="Add transaction" href={{ pathname: "/ledgers/[ledgerId]/new", params: { ledgerId } }} />}>
      <Card>
        <CardContent className="gap-5">
          <LedgerBalance ledgerId={ledgerId} size="large" version={balanceVersion} />
          <View className="border-t border-border pt-4">
            <SafeToSpend ledgerId={ledgerId} version={balanceVersion} />
          </View>
        </CardContent>
      </Card>
      <View className="flex-row justify-around">
        <QuickAction
          icon={ArrowLeftRight}
          label="Transfer"
          href={{ pathname: "/ledgers/[ledgerId]/transfer", params: { ledgerId } }}
        />
        <QuickAction icon={ArrowUpDown} label="Move" href={{ pathname: "/ledgers/[ledgerId]/move", params: { ledgerId } }} />
        {isOwner && (
          <QuickAction
            icon={Scale}
            label="Reconcile"
            href={{ pathname: "/ledgers/[ledgerId]/reconcile", params: { ledgerId } }}
          />
        )}
      </View>
      <WrapUpCard ledgerId={ledgerId} />
      <WeeklyReviewCard ledgerId={ledgerId} />
      <DuePayments ledgerId={ledgerId} onChanged={() => setBalanceVersion((v) => v + 1)} />
      <ErrorMessage message={error ?? categoryList.error ?? potList.error} onRetry={reload} />
      {loading && !transactions && <Skeleton className="h-40 w-full rounded-2xl" />}
      {transactions?.length === 0 && (
        <EmptyState icon={ReceiptText} title="No transactions yet" text="Tap “Add transaction” to book the first one." />
      )}
      {transactions &&
        groupByDay(transactions).map((group, index) => (
          <BlurFade key={group.day} index={index}>
            <ListSection title={dayHeading(group.day)}>
              {group.items.map((tx) => (
                <TransactionRow
                  key={tx.id}
                  tx={tx}
                  categories={categoryList.data ?? []}
                  pots={potList.data ?? []}
                  showAuthor={showAuthor}
                  kind={kind}
                />
              ))}
            </ListSection>
          </BlurFade>
        ))}
      {list.hasMore && (
        <Button variant="outline" onPress={list.loadMore} disabled={list.loadingMore}>
          <Text>{list.loadingMore ? "Loading…" : "Load more"}</Text>
        </Button>
      )}
    </Screen>
  );
}

/** A round button with a caption, for the few actions used often (as in banking apps). */
function QuickAction({
  icon,
  label,
  href,
}: {
  icon: LucideIcon;
  label: string;
  href: React.ComponentProps<typeof Link>["href"];
}) {
  return (
    <Link href={href} asChild>
      <Pressable role="link" className="w-20 items-center gap-1.5 active:opacity-70">
        <View className="h-12 w-12 items-center justify-center rounded-full bg-accent">
          <Icon as={icon} size={20} className="text-accent-foreground" />
        </View>
        <Text className="text-xs font-medium">{label}</Text>
      </Pressable>
    </Link>
  );
}
