// Public surface of the API layer. Generated artifacts (types + hooks)
// are re-exported through here so feature code never imports from the
// `generated/` path directly — that lets us swap the codegen plugin
// or move output around without touching every call site.

export { queryClient } from "./queryClient";

export * from "./generated/types";
export * from "./generated/hooks";
