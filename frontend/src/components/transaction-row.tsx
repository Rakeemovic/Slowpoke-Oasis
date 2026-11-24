import { Zap } from "lucide-react-native";
import { View } from "react-native";

import type { Category, Pot, Transaction } from "@/api/types";
import { Amount } from "@/components/amount";
import { ListRow } from "@/components/list";
import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";
import { hueFor, NEUTRAL_HUE } from "@/lib/hues";
import { categoryIcon, CORRECTION_ICON, TRANSFER_ICON } from "@/lib/icons";
import { potSummary, transactionTitle } from "@/lib/transactions";

/**
 * One booking in a list. The icon and colour come from the category, so the
 * eye finds "Groceries" without reading. Rows the user cannot change open
 * nothing, so they are not pressable.
 */
export function TransactionRow({
  tx,
  categories,
  pots,
  showAuthor,
  kind,
}: {
  tx: Transaction;
  categories: Category[];
  pots: Pot[];
  showAuthor: boolean;
  kind?: string;
}) {
  // The API sends signed amounts: money coming in is positive, money going out
  // negative. A transfer between pots of the same ledger changes neither.
  const isTransfer = tx.type === "transfer";
  const title = transactionTitle(tx, categories);
  const category = categories.find((c) => c.id === tx.category_id);
  const details = [potSummary(tx, pots), showAuthor && (tx.username ?? "Former member"), tx.note].filter(Boolean).join(" · ");
  const icon = tx.type === "correction" ? CORRECTION_ICON : category ? categoryIcon(category.name) : TRANSFER_ICON;
  const hue = category ? hueFor(category.id) : NEUTRAL_HUE;

  return (
    <ListRow
      icon={icon}
      hue={hue}
      title={
        <View className="flex-row items-center gap-1">
          <Text className="shrink font-medium" numberOfLines={1}>
            {title}
          </Text>
          {tx.is_spontaneous && (
            <View className="flex-row items-center" aria-label="Spontaneous">
              <Icon as={Zap} size={14} className="text-warning" />
            </View>
          )}
        </View>
      }
      subtitle={details || undefined}
      trailing={<Amount cents={isTransfer ? Math.abs(tx.amount_cents) : tx.amount_cents} signed={!isTransfer} />}
      href={
        tx.can_edit || tx.can_delete
          ? {
              pathname: "/ledgers/[ledgerId]/transactions/[transactionId]",
              params: { ledgerId: tx.ledger_id, transactionId: tx.id, ...(kind && { kind }) },
            }
          : undefined
      }
    />
  );
}
