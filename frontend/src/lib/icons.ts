import {
  ArrowLeftRight,
  Banknote,
  Bus,
  CircleDashed,
  Coins,
  HeartPulse,
  House,
  Landmark,
  type LucideIcon,
  PiggyBank,
  Plug,
  Repeat,
  Scale,
  Shield,
  Shirt,
  ShoppingBasket,
  Sofa,
  Sparkles,
  Tag,
  Ticket,
  UtensilsCrossed,
  Wallet,
} from "lucide-react-native";

import type { Pot } from "@/api/types";

/**
 * Icons for the default categories (see the backend's defaultCategories).
 * Categories are matched by their name, so a renamed or a new category falls
 * back to a plain tag: the icon is decoration and never carries meaning alone.
 */
const CATEGORY_ICONS: Record<string, LucideIcon> = {
  salary: Banknote,
  "other income": Coins,
  groceries: ShoppingBasket,
  rent: House,
  utilities: Plug,
  transport: Bus,
  insurance: Shield,
  health: HeartPulse,
  "eating out": UtensilsCrossed,
  leisure: Ticket,
  clothing: Shirt,
  subscriptions: Repeat,
  household: Sofa,
  other: CircleDashed,
};

export function categoryIcon(name: string | undefined): LucideIcon {
  return (name && CATEGORY_ICONS[name.trim().toLowerCase()]) || Tag;
}

/** Pots get an icon by their role: main account, real sub-account, reserve, goal or plain pot. */
export function potIcon(pot: Pick<Pot, "is_default" | "is_separate_account" | "reserve_for">, goal = false): LucideIcon {
  if (pot.is_default) {
    return Wallet;
  }
  if (pot.is_separate_account) {
    return Landmark;
  }
  if (pot.reserve_for) {
    return Repeat;
  }
  return goal ? Sparkles : PiggyBank;
}

export const TRANSFER_ICON = ArrowLeftRight;
export const CORRECTION_ICON = Scale;
