# Stack: Node + Puppeteer on ECS

Browser-automation tasks (scheduled bookings, scraping) that run as Fargate
tasks. Bootstrap:

```bash
npm init -y
npm i puppeteer-core && npm i -D typescript esbuild @types/node
```

Use `puppeteer-core`, never `puppeteer`: the runtime image
(`ghcr.io/puppeteer/puppeteer`) already carries Chromium, and connecting to it
keeps the build thin.

## Fitting the Makefile contract

- `check` — prettier, eslint, `tsc --noEmit`.
- `test` — vitest; keep browser code behind interfaces so units run without
  Chromium.
- `build` — bundle to `dist/main.js` (the Dockerfile's CMD).
- `deploy` — the deploy fragment builds the Dockerfile at the repo root and
  registers the task definition; the image entrypoint is the whole contract.
