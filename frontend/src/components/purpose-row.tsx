import { ArrowDown, ArrowUp, Equal, Zap, type LucideIcon } from "lucide-react-native";
import { View } from "react-native";

import type { PurposeLine } from "@/api/types";
import { Amount } from "@/components/amount";
import { IconTile } from "@/components/icon-tile";
import { ProgressRow } from "@/components/progress-row";
import { Card } from "@/components/ui/card";
import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";
import { purposeLook, shareProgress, shareText, spontaneousText, transferText, trendOf, type Trend } from "@/lib/purposes";

const trendIcon: Record<Trend["direction"], LucideIcon> = { up: ArrowUp, down: ArrowDown, same: Equal };

/**
 * One purpose: its amount, its share as the row's own bar, and what changed
 * since the span named by `comparedWith`, e.g. "October".
 */
export function PurposeRow({ line, comparedWith }: { line: PurposeLine; comparedWith: string }) {
  const look = purposeLook[line.purpose];
  const share = shareText(line);
  const trend = trendOf(line, comparedWith);
  const spontaneous = spontaneousText(line);
  const transfer = transferText(line);
  return (
    <Card className="overflow-hidden py-0">
      <ProgressRow progress={shareProgress(line)} label={look.name} fillClassName={look.fill} className="gap-2 p-4">
        <View className="flex-row items-center gap-3">
          <IconTile icon={look.icon} hue={look.hue} />
          <View className="flex-1 gap-0.5">
            <Text className="font-medium">{look.name}</Text>
            {share && <Text variant="muted">{share}</Text>}
          </View>
          <Amount cents={line.cents} size="lg" />
        </View>
        {trend && <DetailLine icon={trendIcon[trend.direction]} text={trend.text} />}
        {spontaneous && <DetailLine icon={Zap} text={spontaneous} />}
        {transfer && <Text variant="muted" className="text-sm">{transfer}</Text>}
      </ProgressRow>
    </Card>
  );
}

function DetailLine({ icon, text }: { icon: LucideIcon; text: string }) {
  return (
    <View className="flex-row items-center gap-1.5">
      <Icon as={icon} size={14} className="text-muted-foreground" />
      <Text variant="muted" className="text-sm">
        {text}
      </Text>
    </View>
  );
}
