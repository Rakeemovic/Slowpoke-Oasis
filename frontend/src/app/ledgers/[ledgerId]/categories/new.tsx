import { router, useLocalSearchParams } from "expo-router";

import { ledgers } from "@/api/endpoints";
import { CategoryForm, type CategoryFormValues } from "@/components/category-form";
import { Screen } from "@/components/screen";

export default function NewCategoryScreen() {
  const { ledgerId } = useLocalSearchParams<{ ledgerId: string }>();

  async function create(values: CategoryFormValues) {
    await ledgers.createCategory(ledgerId, values);
    router.back();
  }

  return (
    <Screen>
      <CategoryForm onSubmit={create} />
    </Screen>
  );
}
