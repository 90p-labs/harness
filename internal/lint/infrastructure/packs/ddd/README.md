# Pack ddd

This pack checks the layout and the code of a domain driven design project.

- A context has the folders `domain`, `application` and `infrastructure`.
- The harness engine checks the folders, the file suffixes and the imports.
- Biome checks events, aggregates, value objects and command and query handlers.
- Each rule has an id, such as `ddd/event-readonly`, and a severity.
- Set `lint.ddd.root` for the business root. The default is `contexts`.
- Set `lint.ddd.domain_allow` to allow a package in the domain.
