import { Moon, Sun } from "lucide-react-native";
import { useColorScheme } from "nativewind";

import { Button } from "@/components/ui/button";
import { Icon } from "@/components/ui/icon";
import { chooseScheme } from "@/lib/color-scheme";

/** Header button that switches between the dark (default) and the light scheme. */
export function ThemeToggle() {
  const { colorScheme } = useColorScheme();
  const next = colorScheme === "light" ? "dark" : "light";
  return (
    <Button variant="ghost" size="icon" onPress={() => chooseScheme(next)} aria-label={`Switch to ${next} mode`}>
      <Icon as={next === "light" ? Sun : Moon} size={20} />
    </Button>
  );
}
