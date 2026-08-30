FROM golang:1.25-alpine AS build
WORKDIR /src
# go.sum may not exist before the first dependency
COPY go.* ./
RUN go mod download
COPY . .
# static binary; the runtime image has no libc
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /app ./cmd/{{PROJECT}}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /app /app
ENTRYPOINT ["/app"]
