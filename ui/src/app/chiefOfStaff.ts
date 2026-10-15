import { createContext, useContext } from "react";

// Opens the Chief of Staff drawer from anywhere (the shell holds it).
export const ChiefOfStaffContext = createContext<() => void>(() => {});

export function useOpenChiefOfStaff(): () => void {
  return useContext(ChiefOfStaffContext);
}
