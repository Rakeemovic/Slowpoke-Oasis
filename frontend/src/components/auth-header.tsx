import { TreePalm } from "lucide-react-native";
import { View } from "react-native";

import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";

/** The app's mark and a short greeting above the sign-in, register and recovery forms. */
export function AuthHeader({ title, text }: { title: string; text: string }) {
  return (
    <View className="items-center gap-4 pb-2 pt-6">
      <View
        aria-hidden
        className="h-16 w-16 rotate-[-6deg] items-center justify-center rounded-3xl bg-primary shadow-lg shadow-primary/30">
        <Icon as={TreePalm} size={32} className="text-primary-foreground" />
      </View>
      <View className="items-center gap-1">
        <Text variant="overline" className="text-primary">
          Slowpoke Oasis
        </Text>
        <Text variant="h1" className="text-center">
          {title}
        </Text>
        <Text variant="muted" className="text-center text-base">
          {text}
        </Text>
      </View>
    </View>
  );
}
