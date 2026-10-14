GATEWAY_MODULE := github.com/aoztrk82/mumweb/gateway
COMPOSE_DEV    := docker compose -f deploy/docker-compose.dev.yml
IMAGE          ?= mumweb:dev
DOCKER_REPO    ?= aoztrk82/mumweb
VERSION        ?= 0.2.0
BUILDER        ?= mumweb-builder

.PHONY: proto web gateway dev docker publish hub-readme test

## proto: regenerate Go code from gateway/proto/*.proto (needs protoc + protoc-gen-go)
proto:
	cd gateway && protoc -I proto --go_out=. --go_opt=module=$(GATEWAY_MODULE) proto/Mumble.proto proto/MumbleUDP.proto

## web: build the Svelte SPA into gateway/web/dist
web:
	cd web && npm ci && npm run build

## gateway: build bin/mumweb and bin/mumprobe with the SPA embedded
gateway: web
	cd gateway && go build -o ../bin/ ./cmd/...

## dev: start Mumble server + gateway via docker compose (needs deploy/.env)
dev:
	$(COMPOSE_DEV) up --build -d
	$(COMPOSE_DEV) ps

## docker: build the production image
docker:
	docker build -t $(IMAGE) .

## publish: build amd64+arm64 and push to Docker Hub (needs `docker login` and a docker-container buildx builder, see docs/BUILDING.md)
publish:
	docker buildx build --builder $(BUILDER) --platform linux/amd64,linux/arm64 -t $(DOCKER_REPO):latest -t $(DOCKER_REPO):$(VERSION) --push .

## hub-readme: update the Docker Hub description from docs/DOCKERHUB.md (uses the docker login of Docker Desktop)
hub-readme:
	python scripts/dockerhub-description.py docs/DOCKERHUB.md

## test: Go tests + vitest
test:
	cd gateway && go test ./...
	cd web && npm test
