import { createContext, useContext } from "react";
import type { CosEntry } from "../api/client";

// Opens the Chief of Staff drawer from anywhere (the shell holds it); an entry
// point starts its flow (wiki/product/features/chief-of-staff.md#drawer).
export const ChiefOfStaffContext = createContext<(entry?: CosEntry) => void>(() => {});

export function useOpenChiefOfStaff(): (entry?: CosEntry) => void {
  return useContext(ChiefOfStaffContext);
}
