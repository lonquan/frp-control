export GO111MODULE=on
LDFLAGS := -s -w
NOWEB_TAG = $(shell [ ! -d web/frps/dist ] || [ ! -d web/frpc/dist ] && echo ',noweb')
FRP_VERSION ?=
VCS_REF ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)

.PHONY: web web-ci frps-web frpc-web frps frpc docker-build docker-build-cn docker-build-frps docker-build-frps-cn docker-build-frpc docker-build-frpc-cn docker-entrypoint-test generate-dns-providers check-dns-providers

all: env fmt web build

build: frps frpc

env:
	@go version

web: frps-web frpc-web

web-ci:
	cd web && npm ci && npm run lint:check --workspace frps && npm run lint:check --workspace frpc && npm run test:unit && npm run build --workspace frps && npm run build --workspace frpc

docker-build: docker-build-frps docker-build-frpc

docker-build-cn: docker-build-frps-cn docker-build-frpc-cn

docker-build-frps:
	FRP_VERSION="$(FRP_VERSION)" VCS_REF="$(VCS_REF)" docker compose -f deploy/frps/compose.yaml build

docker-build-frps-cn:
	FRP_VERSION="$(FRP_VERSION)" VCS_REF="$(VCS_REF)" docker compose --env-file deploy/build.cn.env -f deploy/frps/compose.yaml build

docker-build-frpc:
	FRP_VERSION="$(FRP_VERSION)" VCS_REF="$(VCS_REF)" docker compose -f deploy/frpc/compose.yaml build

docker-build-frpc-cn:
	FRP_VERSION="$(FRP_VERSION)" VCS_REF="$(VCS_REF)" docker compose --env-file deploy/build.cn.env -f deploy/frpc/compose.yaml build

docker-entrypoint-test:
	sh dockerfiles/entrypoint_test.sh

frps-web:
	$(MAKE) -C web/frps build

frpc-web:
	$(MAKE) -C web/frpc build

fmt:
	go fmt ./...

fmt-more:
	gofumpt -l -w .

gci:
	gci write -s standard -s default -s "prefix(github.com/fatedier/frp/)" ./

vet:
	go vet -tags "$(NOWEB_TAG)" ./...

frps:
	env CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -tags "frps$(NOWEB_TAG)" -o bin/frps ./cmd/frps

frpc:
	env CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -tags "frpc$(NOWEB_TAG)" -o bin/frpc ./cmd/frpc

test: check-dns-providers gotest

gotest:
	go test -tags "$(NOWEB_TAG)" -v --cover ./assets/...
	go test -tags "$(NOWEB_TAG)" -v --cover ./cmd/...
	go test -tags "$(NOWEB_TAG)" -v --cover ./client/...
	go test -tags "$(NOWEB_TAG)" -v --cover ./server/...
	go test -tags "$(NOWEB_TAG)" -v --cover ./pkg/...
	cd cmd/frp-acme && go test -v --cover ./...

generate-dns-providers:
	cd cmd/frp-acme && go run ./cmd/generate-provider-registry

check-dns-providers:
	cd cmd/frp-acme && go run ./cmd/generate-provider-registry --check

e2e: build
	./hack/run-e2e.sh

e2e-trace: build
	DEBUG=true LOG_LEVEL=trace ./hack/run-e2e.sh

alltest: vet gotest docker-entrypoint-test e2e
	
clean:
	rm -rf ./bin ./web/frpc/dist ./web/frps/dist ./.cache
