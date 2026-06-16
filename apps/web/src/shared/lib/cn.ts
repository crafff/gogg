import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// `cn` is the single canonical class-name composer for apps/web.
// clsx handles conditional/array shapes; tailwind-merge dedupes
// conflicting Tailwind utilities so `cn('p-2', props.className)` does
// the right thing when callers override with `p-4`.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
