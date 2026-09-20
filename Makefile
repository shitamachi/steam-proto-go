.PHONY: generate test integration-test

generate:
	buf generate --template buf.gen.yaml

test:
	go test -race ./...

# Test this SDK against the sibling API without committing a local replace.
integration-test:
	@set -eu; work=$$(mktemp -d); trap 'rm -rf "$$work"' EXIT; \
		export GOWORK="$$work/go.work"; \
		go work init . ../steam-api; \
		go -C ../steam-api test -race ./internal/server
