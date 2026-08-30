# Stack: Go

Bootstrap the module so the Makefile contract holds from the first commit:

```
go mod init <module-path>
mkdir -p cmd/{{PROJECT}}
```

The entrypoint is `cmd/{{PROJECT}}/main.go`; `make build` compiles it to
`bin/{{PROJECT}}`. Library code goes in `internal/` packages beside it.

## Targets

- `setup` — `go mod download`; commit `go.mod` and `go.sum`.
- `check` — `gofmt -l` must print nothing, and `go vet ./...` must pass.
  Run `gofmt -w .` before committing.
- `test` — `go test -race ./...` with coverage; unit tests only, no
  network, no credentials.
- Integration tests carry `//go:build integration`, so plain `go test`
  never touches them.
