import { useId, type ComponentProps } from "react";
import { View } from "react-native";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Text } from "@/components/ui/text";
import { cn } from "@/lib/utils";

type FormFieldProps = ComponentProps<typeof Input> & { label: string; hint?: string };

/** A labelled text input. The label is linked to the input for screen readers; `hint` explains it below. */
export function FormField({ label, hint, ...inputProps }: FormFieldProps) {
  const id = useId();
  return (
    <View className="gap-1.5">
      <Label nativeID={id}>{label}</Label>
      <Input aria-labelledby={id} {...inputProps} />
      {hint && (
        <Text variant="muted" className="text-xs">
          {hint}
        </Text>
      )}
    </View>
  );
}

/**
 * The amount of a booking or a move, the most important input of its form:
 * large, with the currency beside it, and the number keyboard on phones.
 */
export function AmountField({ label, className, ...inputProps }: FormFieldProps) {
  const id = useId();
  return (
    <View className="gap-1.5">
      <Label nativeID={id}>{label}</Label>
      <View className="flex-row items-center rounded-2xl border border-input bg-card px-4 dark:bg-input/30">
        <Input
          aria-labelledby={id}
          keyboardType="decimal-pad"
          placeholder="0,00"
          className={cn(
            "h-16 flex-1 border-0 bg-transparent px-0 font-display text-3xl shadow-none dark:bg-transparent",
            className,
          )}
          {...inputProps}
        />
        <Text className="font-display text-2xl text-muted-foreground">€</Text>
      </View>
    </View>
  );
}
