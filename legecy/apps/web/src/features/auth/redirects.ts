export function safeReturnTo(raw: string | null): string {
  if (
    !raw ||
    !raw.startsWith("/") ||
    raw.startsWith("//") ||
    raw.includes("\\")
  ) {
    return "/me";
  }
  try {
    const parsed = new URL(raw, window.location.origin);
    if (parsed.origin !== window.location.origin) return "/me";
    return `${parsed.pathname}${parsed.search}${parsed.hash}`;
  } catch {
    return "/me";
  }
}
