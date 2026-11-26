import { ArrowLeftRight, ArrowUpDown, ChartPie, Repeat, Scale, Tags, Trash2, Users } from "lucide-react-native";

import { ListRow, ListSection } from "@/components/list";
import { Screen } from "@/components/screen";
import { hueFor } from "@/lib/hues";
import { useLedger } from "@/lib/ledger-context";

/**
 * Everything about a ledger that is not needed every day, grouped by what it
 * is for. Each row says in its subtitle what the screen does, so nobody has
 * to open it to find out.
 */
export default function MoreScreen() {
  const { ledgerId, kind, householdId, isOwner } = useLedger();
  return (
    <Screen>
      <ListSection title="Move money">
        <ListRow
          title="Transfer"
          subtitle="Send money to another ledger"
          icon={ArrowLeftRight}
          hue={hueFor("transfer")}
          href={{ pathname: "/ledgers/[ledgerId]/transfer", params: { ledgerId } }}
          chevron
        />
        <ListRow
          title="Move between pots"
          subtitle="Set money aside or take it back"
          icon={ArrowUpDown}
          hue={hueFor("move")}
          href={{ pathname: "/ledgers/[ledgerId]/move", params: { ledgerId } }}
          chevron
        />
        {isOwner && (
          <ListRow
            title="Reconcile"
            subtitle="Compare with your bank balance"
            icon={Scale}
            hue={hueFor("reconcile")}
            href={{ pathname: "/ledgers/[ledgerId]/reconcile", params: { ledgerId } }}
            chevron
          />
        )}
      </ListSection>
      <ListSection title="Set up">
        <ListRow
          title="Recurring"
          subtitle="Rent, subscriptions and other regular payments"
          icon={Repeat}
          hue={hueFor("recurring")}
          href={{ pathname: "/ledgers/[ledgerId]/recurring", params: { ledgerId } }}
          chevron
        />
        <ListRow
          title="Categories"
          subtitle="Rename, archive or delete categories"
          icon={Tags}
          hue={hueFor("categories")}
          href={{ pathname: "/ledgers/[ledgerId]/categories", params: { ledgerId } }}
          chevron
        />
        {householdId && (
          <ListRow
            title="Members"
            subtitle="Who shares this household"
            icon={Users}
            hue={hueFor("members")}
            href={{ pathname: "/households/[householdId]/members", params: { householdId } }}
            chevron
          />
        )}
      </ListSection>
      <ListSection title="History">
        <ListRow
          title="Insights"
          subtitle="Where the money went: needs, wants and savings"
          icon={ChartPie}
          hue={hueFor("insights")}
          href={{ pathname: "/ledgers/[ledgerId]/insights", params: { ledgerId } }}
          chevron
        />
        <ListRow
          title="Deleted transactions"
          subtitle="Restore within 30 days"
          icon={Trash2}
          hue={hueFor("deleted")}
          href={{ pathname: "/ledgers/[ledgerId]/deleted", params: { ledgerId, ...(kind && { kind }) } }}
          chevron
        />
      </ListSection>
    </Screen>
  );
}
