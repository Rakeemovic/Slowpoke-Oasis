import { createContext, useContext, type ReactNode } from "react";

/** The ledger whose tabs are open. Provided by ledgers/[ledgerId]/(tabs)/_layout.tsx. */
export interface OpenLedger {
  ledgerId: string;
  name: string;
  kind?: "personal" | "household";
  householdId?: string;
  /** Unknown (undefined) until the ledger list has loaded. */
  isOwner?: boolean;
}

const LedgerContext = createContext<OpenLedger | undefined>(undefined);

export function LedgerProvider({ value, children }: { value: OpenLedger; children: ReactNode }) {
  return <LedgerContext.Provider value={value}>{children}</LedgerContext.Provider>;
}

/**
 * The open ledger, for the tab screens. Search parameters such as the name do
 * not survive switching tabs or reloading the page, so the layout looks the
 * ledger up once and shares it here.
 */
export function useLedger(): OpenLedger {
  const ledger = useContext(LedgerContext);
  if (!ledger) {
    throw new Error("useLedger must be used inside the ledger tabs");
  }
  return ledger;
}
