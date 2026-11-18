import { useId, type ComponentProps } from "react";
import { View } from "react-native";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

type FormFieldProps = ComponentProps<typeof Input> & { label: string };

/** A labelled text input. The label is linked to the input for screen readers. */
export function FormField({ label, ...inputProps }: FormFieldProps) {
  const id = useId();
  return (
    <View className="gap-1.5">
      <Label nativeID={id}>{label}</Label>
      <Input aria-labelledby={id} {...inputProps} />
    </View>
  );
}
