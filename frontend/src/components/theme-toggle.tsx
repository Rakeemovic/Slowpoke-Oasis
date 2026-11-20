import { useColorScheme } from "nativewind";

import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";
import { chooseScheme } from "@/lib/color-scheme";

/** Header button that switches between the dark (default) and the light scheme. */
export function ThemeToggle() {
  const { colorScheme } = useColorScheme();
  const next = colorScheme === "light" ? "dark" : "light";

  return (
    <Button variant="ghost" size="sm" onPress={() => chooseScheme(next)} accessibilityLabel={`Switch to ${next} mode`}>
      <Text>{next === "light" ? "Light" : "Dark"}</Text>
    </Button>
  );
}
