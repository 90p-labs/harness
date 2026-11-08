# Pack shadcn

This pack checks a project that uses shadcn components.

- A ui file imports only ui files, the utils alias, Radix, class-variance-authority, lucide-react and React.
- A className uses the `cn` helper, not a template literal.
- A class or a style uses a theme token, not a raw color.
- The file `components.json` sets `tailwind.cssVariables` to true.
- Set `lint.shadcn.ui_allow` to allow one more module in a ui file.
