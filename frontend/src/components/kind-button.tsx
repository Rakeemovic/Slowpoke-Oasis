import { Button } from "@/components/ui/button";
import { Text } from "@/components/ui/text";

/** One of two side-by-side buttons that switch between expense and income. */
export function KindButton({ label, active, onPress }: { label: string; active: boolean; onPress: () => void }) {
  return (
    <Button className="flex-1" variant={active ? "default" : "outline"} onPress={onPress} aria-selected={active}>
      <Text>{label}</Text>
    </Button>
  );
}
