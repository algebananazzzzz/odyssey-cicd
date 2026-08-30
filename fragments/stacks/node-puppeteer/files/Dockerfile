FROM node:22-alpine AS build
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# Chromium and its deps come preinstalled; puppeteer-core attaches to it.
FROM ghcr.io/puppeteer/puppeteer:24.9.0
WORKDIR /app
COPY --from=build /app/dist ./dist
COPY --from=build /app/node_modules ./node_modules
COPY --from=build /app/package.json .
CMD ["node", "dist/main.js"]
