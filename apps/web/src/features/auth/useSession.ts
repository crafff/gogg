import { useMutation, useQueryClient } from "@tanstack/react-query";

import { useMeQuery, type MeQuery } from "@shared/api";

export function useSession() {
  return useMeQuery(undefined, {
    staleTime: 60_000,
    retry: 1,
    refetchOnWindowFocus: true,
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const response = await fetch("/auth/logout", {
        method: "POST",
        credentials: "same-origin",
        headers: { "X-GOGG-CSRF": "1" },
      });
      if (!response.ok) {
        throw new Error(`logout failed with HTTP ${response.status}`);
      }
    },
    onSuccess: () => {
      queryClient.setQueryData<MeQuery>(useMeQuery.getKey(), { me: null });
    },
  });
}
