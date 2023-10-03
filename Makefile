.PHONY: test

test: export CQ_DEST_PG_TEST_CONN=postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
test: build setup-postgres
# we clean the cache to avoid scenarios when we change something in the db and we want to retest without noticing nothing run
	go clean -testcache
	go test -race -timeout 3m ./...

.PHONY: lint
lint:
	golangci-lint run --config ../../.golangci.yml

build:
	go build
	cd cq-source-test && go build

teardown-postgres:
	docker rm -f postgres || true

setup-postgres: teardown-postgres
	docker run -d --name postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres postgres
