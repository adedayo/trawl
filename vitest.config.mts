import { defineConfig } from 'vitest/config';

// Scoped to the CI automation scripts only. Application tests live under `app/`
// and run through their own Vitest configuration. Explicit `include` is required
// because `.github` is a dotted directory and is not matched by default globs.
export default defineConfig({
  test: {
    include: ['.github/scripts/**/*.test.mjs'],
    environment: 'node',
  },
});
