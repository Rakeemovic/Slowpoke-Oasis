import { Link, type Href } from "expo-router";
import { Plus } from "lucide-react-native";

import { Button } from "@/components/ui/button";
import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";

/**
 * The main "create" action of a list screen, floating at the bottom right
 * where the thumb rests (Material's extended FAB). Pass it to Screen's
 * `floating`. The label is always written out: an icon alone is ambiguous.
 */
export function FloatingAction({ href, label }: { href: Href; label: string }) {
  return (
    <Link href={href} asChild>
      <Button size="lg" className="shadow-lg shadow-black/25" aria-label={label}>
        <Icon as={Plus} size={22} className="text-primary-foreground" />
        <Text>{label}</Text>
      </Button>
    </Link>
  );
}
