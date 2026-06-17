# Chapter 11 · Frontend essentials (TS + React + modern web)

> Goal: by the end of this chapter you understand the React rendering model, the rules of hooks, the TypeScript inference patterns that make hooks ergonomic, when to use which state management approach, the basics of accessibility, and the testing philosophy ("test behavior, not implementation"). All grounded in examples from `apps/web/`.

Read this if you've used vanilla JS but are new to React + TypeScript. If you already know React, skim for the patterns specific to this codebase + the "don't do this" callouts.

## The React mental model in 5 bullets

1. **UI = function(state).** A component is a function that takes props + state and returns JSX (HTML-like markup). React calls it when state changes, gets a tree, diffs it against the previous tree, mutates the DOM.
2. **Re-renders are cheap.** A component re-renders every time its state or props change. That's the price of admission; don't fight it. Optimize only when profiling proves a hot path.
3. **State lives in components.** Either local (`useState` inside the component) or shared (lifted up to a parent, or via a provider). React doesn't have a global state store baked in.
4. **Hooks are how you reach state + lifecycle.** `useState`, `useEffect`, `useMemo`, `useRef`, `useCallback`, plus custom hooks (any function starting with `use`). Hooks let function components do everything class components used to.
5. **Components compose; logic in hooks.** Components are small + declarative. Reusable logic (data fetching, state machines, animations) lives in custom hooks the components consume.

If you internalize "UI = function(state)" and "re-renders are cheap unless proven otherwise," you've got 80% of React.

## TypeScript without tears

TypeScript = JavaScript + a structural type system that's erased at runtime. The compiler verifies types; the runtime is plain JS.

### Inference does a lot

```tsx
const [count, setCount] = useState(0);   // count: number, setCount: (n: number) => void
```

You wrote `0`; TS inferred `number`. Same for arrays, objects, function returns. **Don't over-annotate** — write the value, let TS infer.

When inference isn't enough:

```tsx
const [user, setUser] = useState<User | null>(null);
```

You needed to be explicit because `null` alone doesn't tell TS what shape `user` becomes after `setUser({...})`.

### `strict` mode

`tsconfig.app.json` has `"strict": true` + `"noUncheckedIndexedAccess": true`. The latter means:

```ts
const xs = [1, 2, 3];
const first = xs[0];     // type: number | undefined  (because xs[100] would be undefined)
```

You're forced to check for `undefined`. Annoying at first, prevents real bugs. Embrace it.

### Discriminated unions

When something can be one of N shapes:

```ts
type LoadState<T> =
    | { status: "idle" }
    | { status: "loading" }
    | { status: "success"; data: T }
    | { status: "error"; error: Error };

function render(s: LoadState<User>) {
    switch (s.status) {
        case "success": return <p>{s.data.name}</p>;   // TS knows .data exists
        case "error":   return <p>{s.error.message}</p>;
        // ...
    }
}
```

TS narrows the type inside each branch. This is the "Make impossible states unrepresentable" pattern — you can't accidentally read `s.data` while loading because the union doesn't have it there.

`useFadeTransition` uses this for the 4-phase state.

### Type vs interface

```ts
type X = { a: number };       // alias
interface Y { a: number; }    // declaration
```

Both work. Convention in this codebase: `type` for unions, primitives, mapped types; `interface` for object shapes you might extend. The two are 90% interchangeable. Prefer `type` if you don't need declaration-merging.

### Generics

```ts
function identity<T>(x: T): T {
    return x;
}

const n = identity(42);       // T inferred as number
const s = identity("hello");  // T inferred as string
```

In hooks:

```ts
function useInfiniteScroll<T extends HTMLElement>({ ... }): RefObject<T> {
    const ref = useRef<T>(null);
    return ref;
}

// Caller:
const ref = useInfiniteScroll<HTMLDivElement>({...});  // RefObject<HTMLDivElement>
```

The `<T extends HTMLElement>` constraint says "T must be some HTML element type." That lets the hook be reused for `<div>` or `<button>` or `<table>` sentinels.

🛠️ **Exercise**: open `apps/web/src/features/rankings/hooks/useInfiniteScroll.ts`. Notice `useRef<T>(null)` — that's how you tell TS "this ref will hold a `T` eventually, even though it starts as null." The return type `RefObject<T>` is what makes JSX's `ref={ref}` accept it.

## React essentials

### useState

```tsx
const [count, setCount] = useState(0);
```

`setCount(5)` schedules a re-render with `count = 5`. The current `count` value won't change in this render — React batches updates and applies them on the next render.

When the new value depends on the old:

```tsx
setCount(prev => prev + 1);     // function form is race-safe
```

Always use the function form when the new state depends on the old. The non-function form (`setCount(count + 1)`) uses a stale closure if called multiple times before React commits.

### useEffect

The hook for "do something after render," typically I/O or subscribing to external state:

```tsx
useEffect(() => {
    const interval = setInterval(() => tick(), 1000);
    return () => clearInterval(interval);   // cleanup runs before next effect + on unmount
}, [tick]);                                 // deps array — effect re-runs if any dep changes
```

The cleanup function (returned from the effect) runs:

1. Before the effect runs again (because deps changed)
2. When the component unmounts

This is how you avoid leaking timers, observers, subscriptions.

#### The deps array gotcha

```tsx
useEffect(() => {
    fetchData(filter);
}, []);   // empty array → run once on mount
```

This passes lint... but if `filter` later changes, the effect won't re-run. ESLint's `react-hooks/exhaustive-deps` rule flags this.

Rule of thumb:

- ✅ Include every reactive value the effect uses in deps.
- ❌ Don't lie about deps to avoid re-runs; restructure instead.
- ✅ For "run once," guard with `useRef` or move into a non-effect.

In this codebase the `eslint-disable-next-line react-hooks/exhaustive-deps` lives in exactly one place: `RankingsPage.tsx`'s phase-commit effect. The comment next to it explains why; chunk 5's reducer refactor will eliminate it.

### useRef

Two uses:

1. **DOM refs**: `const ref = useRef<HTMLDivElement>(null); <div ref={ref} />` — then `ref.current` is the DOM node.
2. **Mutable values that don't trigger re-render**: `const timerRef = useRef<number | null>(null); timerRef.current = setTimeout(...)` — refs are persistent across renders but assignments don't cause re-renders.

`useFadeTransition` uses both — viewport ref for height measurement, timer refs for cleanup.

### useMemo + useCallback

Memoization. You **rarely need these.** Optimize only if profiling shows a real problem.

```tsx
const expensiveValue = useMemo(() => heavyComputation(a, b), [a, b]);
const handler = useCallback(() => doThing(a), [a]);
```

`useMemo` caches a value; `useCallback` caches a function reference. Both re-compute when deps change.

When to use:

- Computing something genuinely expensive (sorting a 10k-row table)
- Passing a callback to a memoized child component (preventing its re-render)

When not to use:

- "Just in case" — the memo overhead can exceed the saved work
- Around cheap computations
- For deps stability when there are simpler fixes (move the value outside the component)

### Custom hooks

Any function whose name starts with `use` and can call other hooks. This is how you share stateful logic between components without HOCs or render props.

The convention: a custom hook returns whatever the consumer needs — an object, a tuple, a single value.

```ts
// useRankingsFilters returns an object
return {
    selected,
    committed,
    setPosition,
    setTier,
    commit,
};

// useInfiniteScroll returns just a ref
return sentinelRef;
```

🛠️ **Exercise**: read `apps/web/src/features/rankings/hooks/useFadeTransition.ts`. It's a self-contained custom hook that manages a 4-phase state machine + height locking + timer cleanup. No component logic, no props, no JSX. Pure stateful logic packaged for reuse.

## Hooks rules (don't break these)

1. **Only call hooks at the top level** of a function component or another hook. Not inside `if`, `for`, `try`, callbacks.
2. **Only call hooks from function components or custom hooks** — not from regular functions or class methods.

Why? React identifies hooks by call order. Conditional calls break the order, so React assigns the wrong state to the wrong hook.

ESLint's `react-hooks/rules-of-hooks` enforces this. You'll see errors if you slip.

## React rendering model in depth

When state changes:

1. React calls your component function, gets a new JSX tree.
2. It diffs against the previous tree (the Virtual DOM diff).
3. It applies only the changes to the real DOM.

The diff is per-component. Re-rendering a parent re-renders all children **whose props changed** (deep equality for objects unless memoized).

If you see a perf issue:

1. Profile with React DevTools' Profiler tab.
2. Find the component that re-renders too often.
3. Either lift state up (so siblings stop re-rendering together), or memo the child (`React.memo`), or stabilize the offending prop.

Don't pre-optimize. Most apps never need this.

## State management spectrum

When to put state where:

| Scope | Where | When |
|---|---|---|
| Single component | `useState` | Filter selections, modal open/close, accordion state |
| Sibling components | Lift to parent + pass via props | A few siblings reading from + setting the same value |
| Subtree | React Context + `useContext` | Theme, auth user (rarely changes) |
| Global | Zustand (or similar) | Cross-tree state that ≥3 disjoint subtrees need |
| Server data | TanStack Query / SWR | Anything the server owns (rankings, summoner search) |

**The big mistake**: using `useState` + `useEffect` to fetch server data. That's how you end up with stale data, race conditions, and waterfalls. Use TanStack Query.

### TanStack Query

The pattern:

```tsx
const { data, isLoading, isError } = useChampionRankingsQuery({ filter });
```

What it does:

- Caches the result keyed by the query name + variables
- Returns from cache if fresh (`staleTime`)
- Refetches in background if stale
- De-duplicates concurrent calls from different components
- Handles refetch-on-focus / reconnect (configurable)
- Exposes loading + error states uniformly

You don't write `useEffect(() => fetch(...))`. The hook owns the lifecycle.

GOGG's queryClient defaults: `staleTime: 60s`, `gcTime: 1h`, `refetchOnWindowFocus: false` (rankings is a browse surface, not a dashboard).

### Zustand

For things that are truly client-only state (filter form values you want to persist across navigation, UI preferences):

```ts
const useFilters = create((set) => ({
    position: "",
    setPosition: (p) => set({ position: p }),
}));

// In a component:
const position = useFilters((s) => s.position);
```

Zustand is small + boilerplate-free. The codebase doesn't have a heavy Zustand store yet — Phase E will when user prefs land.

## Composition patterns

### Children + slots

```tsx
function Card({ children }: { children: React.ReactNode }) {
    return <div className="rounded border bg-surface-raised p-4">{children}</div>;
}

// Caller:
<Card>
    <h2>Title</h2>
    <p>Body</p>
</Card>
```

The `children` prop is the canonical "pass arbitrary content here" shape. For multiple "slots":

```tsx
function Layout({ header, sidebar, children }: { ... }) {
    // ...
}
```

### asChild (Radix pattern)

```tsx
<Button asChild>
    <a href="/somewhere">Go</a>
</Button>
```

The `asChild` prop tells `Button` to render its child directly with its styles, instead of rendering a `<button>`. Useful for "I want this to look like a button but be a link."

GOGG's `Button` uses Radix's `Slot` primitive to implement this. Read `apps/web/src/shared/ui/Button.tsx` to see.

### forwardRef

When your component needs to expose its underlying DOM ref to the parent:

```tsx
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
    (props, ref) => <button ref={ref} {...props} />
);
```

The codebase uses this for every base component because `tooltip`, `dropdown`, and similar primitives need to attach refs.

## Accessibility (a11y)

The bare minimum:

- **Use semantic HTML.** `<button>` for buttons, `<a>` for links, `<nav>` for navigation. Don't `<div onClick={...}>` your way through life.
- **Label every form input.** `<label>` wraps or `htmlFor` references.
- **Manage focus.** When opening a modal, focus the first interactive element. When closing, return focus to the trigger.
- **Use ARIA when semantic HTML can't express it.** `aria-pressed`, `aria-expanded`, `aria-current`. Don't add ARIA unnecessarily — wrong ARIA is worse than no ARIA.

Radix Primitives (`@radix-ui/react-*`) give you accessible building blocks for the hard cases: dropdowns, tooltips, dialogs. GOGG's `Select` wraps `@radix-ui/react-select`. You inherit keyboard navigation, screen reader support, focus management for free.

🛠️ **Exercise**: open `apps/web/src/features/rankings/components/RankingsFilters.tsx`. Find the position chips. Notice `aria-pressed={isActive}` — that's how a screen reader announces "Top button, pressed" or "Top button, not pressed." Without it, the toggle state is invisible to non-sighted users.

### Tools

- `eslint-plugin-jsx-a11y` catches common mistakes statically.
- Browser DevTools → Lighthouse runs an accessibility audit.
- macOS VoiceOver / NVDA on Windows — actually try with a screen reader at least once.

## Styling: Tailwind philosophy

Tailwind is "utility-first CSS." Instead of writing CSS, you compose pre-defined utility classes:

```tsx
<div className="rounded-lg border border-border bg-surface-raised p-6 shadow-card">
```

vs CSS Modules:

```css
.card { border-radius: 0.5rem; border: 1px solid ...; ... }
```

Tradeoffs:

- ✅ No naming things (`card`, `cardWrapper`, `cardInner`...).
- ✅ Co-located: the styling is right there in the JSX.
- ✅ Constraints: you can only pick from the design tokens.
- ❌ Long class strings.
- ❌ Different mental model from "real" CSS.

GOGG's `tailwind.config.ts` extends Tailwind with semantic tokens (`surface-raised`, `fg-muted`, `accent`, `border-default`). Every component speaks semantic tokens, not raw colors. That's how the theme stays consistent.

### `cn()` helper

```ts
import { cn } from "@shared/lib/cn";

const className = cn(
    "rounded p-2",
    variant === "primary" && "bg-accent text-fg-inverse",
    className,
);
```

`cn()` composes class strings with `clsx` (handles conditionals) + `tailwind-merge` (deduplicates conflicting Tailwind utilities so `p-2` is overridden by a caller's `p-4`).

Every styled component uses `cn` to compose its variants + caller overrides.

### cva (class-variance-authority)

For variant matrices:

```ts
export const buttonStyles = cva(
    "inline-flex select-none items-center justify-center ...",
    {
        variants: {
            variant: {
                primary: "bg-accent ...",
                secondary: "bg-surface-raised ...",
            },
            size: {
                sm: "h-7 px-2.5 text-xs",
                md: "h-9 px-3.5 text-sm",
            },
        },
        defaultVariants: { variant: "primary", size: "md" },
    },
);
```

Then `buttonStyles({ variant: "secondary", size: "md" })` returns the right class string.

Read `apps/web/src/shared/ui/Button.variants.ts` for the full example.

## Forms

Not deeply exercised in this codebase yet — Phase E adds the real forms (login, summoner search). The intended approach:

- **react-hook-form** for state + validation
- **zod** for schemas (single source of truth: validate + infer TS type)

The combo gives you typed form state without boilerplate.

## Testing philosophy

**Test behavior, not implementation.** The test should fail when the user-visible behavior changes; it should pass when an internal refactor changes the implementation.

### Testing Library

`@testing-library/react` is built around this philosophy. The queries (`getByRole`, `getByLabelText`, `getByText`) match what a user (or screen reader) sees.

```ts
// GOOD: tests user-visible behavior
const button = screen.getByRole("button", { name: "Sign in" });
await userEvent.click(button);
expect(onSubmit).toHaveBeenCalled();

// BAD: tests implementation detail
const button = container.querySelector(".SignInButton");
```

If you refactor the SignInButton class but keep its role + name, the GOOD test still passes.

### Mocking guidelines

- ❌ Don't mock React itself
- ❌ Don't mock components you own
- ✅ Mock the network (`vi.stubGlobal("fetch", ...)`)
- ✅ Mock browser APIs not in jsdom (`IntersectionObserver`)
- ✅ Wrap things needing a provider with a test wrapper (QueryClientProvider)

The `useRankingsQuery` test demonstrates: mock `fetch`, wrap in `QueryClientProvider`, assert the request body. Behavior tested; implementation free to change.

### renderHook

For testing custom hooks in isolation:

```ts
const { result } = renderHook(() => useFadeTransition({ fadeOutMs: 100 }));
act(() => { result.current.beginExit(); });
expect(result.current.phase).toBe("fading-out");
```

`act()` flushes pending state updates before the next assertion. Vitest 4 + React 18 require this for state-changing actions.

### Playwright

End-to-end tests run a real browser against your app:

```ts
await page.goto("/");
await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
await page.getByRole("button", { name: "Top" }).click();
await expect(page.getByText("Champion-TOP-1")).toBeVisible();
```

GOGG mocks the GraphQL endpoint via `page.route("**/graphql", ...)` so the test is hermetic. Use Playwright sparingly — unit tests are 100x faster. One golden-path E2E per critical user flow is the right dose.

## Modern web tooling primer

- **Vite** is the build tool. In dev, it serves your source unbuilt + transforms on demand (instant HMR). In prod, it runs Rollup to bundle + minify. Compared to webpack, it starts faster + has fewer config foot-guns.
- **ESBuild** is the underlying transformer (TS → JS, JSX → JS). Written in Go, ~100x faster than Babel for the same job. Vite invokes it under the hood.
- **TypeScript** is the type-checker. Doesn't transform code at build time in this stack — esbuild handles that — but `tsc -b --noEmit` runs in CI as a strict gate.
- **ESLint** is the linter. Catches React-specific bugs, accessibility issues, code smells. Project uses flat config (v9) with `typescript-eslint` + `react-hooks` + `react-refresh`.
- **Prettier** is the formatter. Run via lefthook pre-commit.

Each tool has one job. You'll see this separation everywhere in the modern web stack.

## Going further

- [React docs (new)](https://react.dev/) — the rewritten docs, focused on hooks. Start with "Learn" → "Describing the UI" → "Adding Interactivity" → "Managing State."
- [Dan Abramov — A Complete Guide to useEffect](https://overreacted.io/a-complete-guide-to-useeffect/) — the single best deep-dive on the hook everyone gets wrong.
- [TypeScript Handbook](https://www.typescriptlang.org/docs/handbook/intro.html) — the official tour. Chapters: Basics, Everyday Types, Narrowing, Generics.
- [Total TypeScript (Matt Pocock)](https://www.totaltypescript.com/tutorials) — free TS challenges + tutorials, very high quality.
- [TanStack Query docs](https://tanstack.com/query/v5/docs/) — start with "Important Defaults" and "Queries."
- [Radix UI](https://www.radix-ui.com/primitives) — for accessible primitives.
- [Tailwind docs](https://tailwindcss.com/docs) — the utility reference.

For modern-web fundamentals:

- [web.dev](https://web.dev/) — Google's guide to the platform. Performance, accessibility, PWAs.
- [MDN](https://developer.mozilla.org/) — the truth source for HTML/CSS/JS APIs.

## Up next

[Chapter 12 — Reading codebases](./12-reading-codebases.md) is the meta-skill chapter: how to onboard quickly to any unfamiliar large codebase, using GOGG's structure as the worked example. Strategies for grep, follow-the-data-flow, "read tests first," and knowing when to give up and ask.
