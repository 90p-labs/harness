# Pack nextjs-ddd-shadcn

This pack joins the layout of Next.js, domain driven design and shadcn. It needs the packs `ddd`, `nextjs` and `shadcn`.

- A component does not import a business root path.
- An app file imports the business root only through `kernel.ts`.
- The `cn` helper is in `src/components/lib/utils.ts`.
