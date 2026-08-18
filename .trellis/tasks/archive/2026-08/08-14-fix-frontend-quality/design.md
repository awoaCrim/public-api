# Design: Frontend Quality Fixes

All changes are mechanical and behavior-preserving:

- braces satisfy the repository control-flow style;
- evidence-list keys use a stable composite derived from evidence content and occurrence count rather than the raw array index;
- imports use `import type` so TypeScript removes them completely at runtime.

No API, component props, state model, or translation contract changes.
