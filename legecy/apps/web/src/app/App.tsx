import { QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "react-router-dom";

import { queryClient } from "@shared/api";
import "@shared/i18n";

import { router } from "./router";

// Top-level provider stack. Layout + the routed page tree live under
// `router`; this component only owns QueryClient + the i18n side
// effect that bootstraps the resources.
export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
}
