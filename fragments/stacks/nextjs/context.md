# Stack: Next.js on Cloudflare Workers

Bootstrap:

```bash
npx create-next-app@latest . --typescript --eslint --app --no-src-dir
npx create-cloudflare@latest . --framework=next --no-deploy
```

The second command wires OpenNext: `open-next.config.ts`, `wrangler.jsonc` and
the `opennextjs-cloudflare` scripts the deploy target calls.

## Fitting the Makefile contract

- `check` — prettier, eslint and `tsc --noEmit` must all pass.
- `test` — vitest; add `vitest.config.ts` and keep tests beside their source.
- `build` — `npm run build` (next build).
- `deploy` — handled by the deploy fragment; per-env Worker config lives in
  `wrangler.jsonc` under `env.<name>`, with prd at the top level.
