# Stack: Astro on Cloudflare Pages

Bootstrap:

```bash
npm create astro@latest . -- --template minimal --typescript strict --no-git
```

## Fitting the Makefile contract

- `check` — prettier, eslint and `astro check` (`@astrojs/check`) must pass;
  keep `tsc --noEmit` working or alias it to `astro check` in package.json.
- `test` — vitest.
- `build` — `npm run build`, output in `dist/` (the deploy fragment's `DIST`
  default).
- `deploy` — handled by the deploy fragment via wrangler; the Pages project is
  named `{env}-web-pages-{project}`.
