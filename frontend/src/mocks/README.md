# Mock data

Everything in this directory is placeholder data for features whose backend does
not exist yet. Nothing here talks to the Kaeru Core API.

A feature graduates out of this directory by:

1. Adding the real endpoints to Kaeru Core.
2. Adding a store under `src/stores/` that calls them.
3. Deleting the matching module here and its imports.

If a component imports from `src/mocks/`, that part of the UI is not wired up.
