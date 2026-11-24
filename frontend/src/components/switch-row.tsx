import { Pressable, View } from "react-native";

import { Switch } from "@/components/ui/switch";
import { Text } from "@/components/ui/text";

/**
 * An on/off setting with a label and an explanation. The whole row toggles,
 * not only the small switch, so it is easy to hit on a phone.
 */
export function SwitchRow({
  label,
  description,
  checked,
  onCheckedChange,
}: {
  label: string;
  description?: string;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  return (
    <Pressable
      onPress={() => onCheckedChange(!checked)}
      className="flex-row items-center gap-4 rounded-2xl border border-border bg-card p-4">
      <View className="flex-1 gap-1">
        <Text className="font-medium">{label}</Text>
        {description && <Text variant="muted">{description}</Text>}
      </View>
      <Switch checked={checked} onCheckedChange={onCheckedChange} aria-label={label} />
    </Pressable>
  );
}
