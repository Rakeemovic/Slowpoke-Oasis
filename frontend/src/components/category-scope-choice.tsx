import { View } from "react-native";

import type { CategoryScope } from "@/api/types";
import { ErrorMessage } from "@/components/error-message";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Text } from "@/components/ui/text";

/**
 * Asks which bookings of a recurring payment get a new category (rule R9).
 * Earlier and later count by the due date the bookings pay. A member changes
 * only their own bookings and cannot reach later ones, because those scopes
 * also change the payment, which only the owner may do.
 */
export function CategoryScopeChoice({
  categoryName,
  isOwner,
  busy,
  error,
  onChoose,
  onCancel,
}: {
  categoryName: string;
  isOwner: boolean;
  busy: boolean;
  error?: string;
  onChoose: (scope: CategoryScope) => void;
  onCancel: () => void;
}) {
  const choices: { scope: CategoryScope; label: string }[] = [
    { scope: "this", label: "Only this booking" },
    { scope: "this_and_earlier", label: "This and earlier ones" },
    ...(isOwner
      ? [
          { scope: "this_and_later" as const, label: "This and later ones" },
          { scope: "all" as const, label: "All of this series" },
        ]
      : []),
  ];

  return (
    <Card>
      <CardContent className="gap-3">
        <Text>This booking belongs to a recurring payment. Which of its bookings move to “{categoryName}”?</Text>
        <Text variant="muted">
          {isOwner
            ? "“Later” and “All” also change the recurring payment, so future bookings use the new category."
            : "Only your own bookings change. Only the owner can change later bookings and the recurring payment."}
        </Text>
        <View className="gap-2">
          {choices.map((c) => (
            <Button key={c.scope} onPress={() => onChoose(c.scope)} disabled={busy}>
              <Text>{c.label}</Text>
            </Button>
          ))}
          <Button variant="outline" onPress={onCancel} disabled={busy}>
            <Text>Cancel</Text>
          </Button>
        </View>
        <ErrorMessage message={error} />
      </CardContent>
    </Card>
  );
}
