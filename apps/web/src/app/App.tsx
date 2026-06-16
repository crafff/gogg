// Phase D chunk 1 App shell: just enough to verify the toolchain end
// to end. The full router + QueryClient + i18n providers land in chunk
// 2 — keeping the root tiny here makes a tailwind/vite plumbing
// failure obvious instead of getting buried in provider noise.
export function App() {
  return (
    <main className="flex min-h-screen flex-col items-center justify-center gap-4 p-8">
      <h1 className="text-4xl font-semibold tracking-tight">GOGG</h1>
      <p
        className="rounded-md border border-gogg-gold/40 bg-gogg-gold/10 px-4 py-2 text-sm"
        data-testid="smoke"
      >
        Phase D chunk 1 toolchain smoke — tailwind + vite + react online.
      </p>
    </main>
  );
}
