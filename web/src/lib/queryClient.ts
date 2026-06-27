import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Keep data fresh for 30 seconds before considering it stale
      staleTime: 30000,
      // Keep unused data in cache for 5 minutes
      gcTime: 300000,
      // Don't refetch on window focus to reduce jitter
      refetchOnWindowFocus: false,
      // Don't refetch on reconnect automatically
      refetchOnReconnect: false,
      // Only retry once on failure
      retry: 1,
      // Retry delay
      retryDelay: 1000,
    },
    mutations: {
      // Retry once on mutation failure
      retry: 1,
    },
  },
})
