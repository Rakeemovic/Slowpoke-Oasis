import { Link, type Href } from "expo-router";
import { ChevronRight, type LucideIcon } from "lucide-react-native";
import { Children, isValidElement, type ReactNode } from "react";
import { Pressable, View } from "react-native";

import { IconTile } from "@/components/icon-tile";
import { Icon } from "@/components/ui/icon";
import { Text } from "@/components/ui/text";
import type { Hue } from "@/lib/hues";
import { cn } from "@/lib/utils";

/**
 * A titled group of rows on one card, the pattern of iOS settings and
 * Material lists: related things sit together, and the title says what they
 * are. `action` is an optional small button on the right of the title.
 */
export function ListSection({
  title,
  action,
  children,
  className,
}: {
  title?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  // Rows are separated by a line, except before the first one.
  const rows = Children.toArray(children).filter(isValidElement);
  return (
    <View className={cn("gap-2", className)}>
      {(title || action) && (
        <View className="min-h-8 flex-row items-center justify-between px-1">
          {title ? (
            <Text variant="overline" role="heading" aria-level={2}>
              {title}
            </Text>
          ) : (
            <View />
          )}
          {action}
        </View>
      )}
      {rows.length > 0 && (
        <View className="overflow-hidden rounded-2xl border border-border bg-card">
          {rows.map((row, index) => (
            <View key={row.key ?? index} className={cn(index > 0 && "border-t border-border")}>
              {row}
            </View>
          ))}
        </View>
      )}
    </View>
  );
}

export interface ListRowProps {
  title: ReactNode;
  subtitle?: ReactNode;
  /** Right side, usually an amount. */
  trailing?: ReactNode;
  icon?: LucideIcon;
  hue?: Hue;
  onPress?: () => void;
  /** Opens another screen; the row is then a link (role "link"). */
  href?: Href;
  /** Shows a chevron: the row opens another screen. */
  chevron?: boolean;
  /** Accessible name when the row is pressable; without it, the row's texts name it. */
  label?: string;
  role?: "link" | "button";
  className?: string;
}

/** One row of a ListSection: icon tile, title and subtitle, trailing value. */
export function ListRow({
  title,
  subtitle,
  trailing,
  icon,
  hue,
  onPress,
  href,
  chevron,
  label,
  role = "button",
  className,
}: ListRowProps) {
  const content = (
    <View className={cn("min-h-14 flex-row items-center gap-3 px-4 py-3", className)}>
      {icon && hue && <IconTile icon={icon} hue={hue} />}
      <View className="flex-1 gap-0.5">
        {typeof title === "string" ? (
          <Text className="font-medium" numberOfLines={1}>
            {title}
          </Text>
        ) : (
          title
        )}
        {typeof subtitle === "string" ? (
          <Text variant="muted" numberOfLines={1}>
            {subtitle}
          </Text>
        ) : (
          subtitle
        )}
      </View>
      {trailing}
      {chevron && <Icon as={ChevronRight} size={18} className="text-muted-foreground" />}
    </View>
  );
  const pressedStyle = "active:bg-accent web:hover:bg-accent/50";
  if (href) {
    return (
      <Link href={href} asChild>
        <Pressable role="link" aria-label={label} className={pressedStyle}>
          {content}
        </Pressable>
      </Link>
    );
  }
  if (!onPress) {
    return content;
  }
  return (
    <Pressable onPress={onPress} role={role} aria-label={label} className={pressedStyle}>
      {content}
    </Pressable>
  );
}
