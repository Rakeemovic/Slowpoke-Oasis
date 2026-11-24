import type { LucideIcon } from "lucide-react-native";
import { View } from "react-native";

import { Icon } from "@/components/ui/icon";
import type { Hue } from "@/lib/hues";
import { cn } from "@/lib/utils";

const SIZES = {
  sm: { box: "h-8 w-8 rounded-lg", icon: 16 },
  md: { box: "h-10 w-10 rounded-xl", icon: 20 },
  lg: { box: "h-12 w-12 rounded-2xl", icon: 24 },
};

/** A coloured rounded square with an icon, the visual anchor of list rows. */
export function IconTile({ icon, hue, size = "md" }: { icon: LucideIcon; hue: Hue; size?: keyof typeof SIZES }) {
  const s = SIZES[size];
  return (
    <View aria-hidden className={cn("items-center justify-center", s.box, hue.tile)}>
      <Icon as={icon} size={s.icon} className={hue.icon} />
    </View>
  );
}
