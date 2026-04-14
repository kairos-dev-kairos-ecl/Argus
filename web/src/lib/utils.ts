import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Utility to merge Tailwind class names safely.
 * Resolves conflicting classes (e.g., p-4 + p-2 → p-2).
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}