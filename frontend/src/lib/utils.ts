import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// cn joins class names and lets later Tailwind classes override earlier ones,
// so components can accept a className that adjusts their default look.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
