.PHONY: tidy
tidy:
	go mod tidy -v
	go fmt ./...
	gofmt -s -w .

wire-install:
	go install github.com/google/wire/cmd/wire@latest

wire:
	wire ./internal

build-web:
	cd web && yarn build

build-server:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o bin/prism cmd/serv/main.go
	upx bin/prism

build:
	make build-web
	make build-server

# Docker 相关命令
.PHONY: docker-build docker-push docker-tag docker-all

# 构建 Docker 镜像
docker-build:
	docker build -t prism:latest .

# 标记镜像为 GitHub Container Registry 格式
docker-tag:
	docker tag prism:latest ghcr.io/dushixiang/prism:latest
	docker tag prism:latest ghcr.io/dushixiang/prism:$(shell git describe --tags --always --dirty)

# 推送镜像到 GitHub Container Registry
docker-push: docker-tag
	docker push ghcr.io/dushixiang/prism:latest
	docker push ghcr.io/dushixiang/prism:$(shell git describe --tags --always --dirty)

# 构建并推送 Docker 镜像（完整流程）
docker-all: docker-build docker-push

start-dev-db:
	docker compose -f docker-compose.dev.yaml up -d

stop-dev-db:
	docker compose -f docker-compose.dev.yaml down