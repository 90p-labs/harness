# Pack nextjs

This pack checks a Next.js App Router project.

- A client file does not import server-only code.
- The folder `src/app` holds only route files and CSS modules.
- A route handler has 30 lines or less.
- The folder `src` has no folder named `lib` at any depth.
- Biome checks the `img` and `head` elements.
- Oxlint checks the page links and the async client components. Biome has no rule for them.
