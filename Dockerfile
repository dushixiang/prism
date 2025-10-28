# 多阶段构建 Dockerfile
# 阶段 1: 构建前端
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend

# 复制前端源码
COPY frontend/ ./
# 安装依赖
RUN npm install
# 构建前端应用
RUN npm run build

# 阶段 2: 构建后端
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app

# 安装构建依赖
RUN apk add --no-cache git gcc musl-dev

# 复制 Go 模块文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源码
COPY . .
# 复制前端编译成果
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist/

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o prism ./cmd/serv

# 阶段 3: 运行时镜像
FROM alpine:latest

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /opt/prism

# 创建非root用户
RUN addgroup -g 1001 -S prism && \
    adduser -S prism -u 1001

# 从构建阶段复制文件
COPY --from=backend-builder /app/prism .

# 复制配置文件模板 (如果存在)
COPY config.yaml* ./

# 创建必要的目录
RUN mkdir -p logs data && \
    chown -R prism:prism /opt/prism

# 切换到非root用户
USER prism

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./prism", "serve", "--config", "config.yaml"]

