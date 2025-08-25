## Prism 部署指南（Docker/Compose）

### 简介

Prism 是一个加密货币分析平台，包含前端与后端一体化服务。本文档介绍如何通过 Docker 与 Docker Compose 部署。

### 系统要求

- 已安装 Docker 以及 Docker Compose（Docker Desktop 自带）
- 可选：一个可用的 PostgreSQL 数据库（若使用内置 Compose，将自动拉起）

### 目录结构与镜像

- 生产镜像：`ghcr.io/dushixiang/prism:latest`
- 服务默认监听：`0.0.0.0:8080`
- 配置文件默认路径：容器内 `/opt/prism/config.yaml`

### 一键启动（推荐，使用 docker-compose.yml）

1. 在项目根目录准备配置文件：复制并根据需要修改配置文件。

    ```bash
    cp config.example.yaml config.yaml
    ```

2. 启动：

    ```bash
    docker compose up -d
    ```

3. 访问：

    ```bash
    open http://localhost:8080
    ```

启动后会创建两个持久化卷：

- `postgresql-data`：PostgreSQL 数据
- `prism-logs`：Prism 日志目录 `/opt/prism/logs`

### 首次创建管理员用户

容器内置 CLI 可管理用户。示例（非交互，直接指定密码）：

```bash
docker compose exec prism ./prism user create \
  -n "Admin" -a admin -p "StrongPass123!" -t admin \
  --config /opt/prism/config.yaml
```

常用用户管理命令：

```bash
# 列表
docker compose exec prism ./prism user list --config /opt/prism/config.yaml | cat

# 修改密码
docker compose exec prism ./prism user change-password -i <USER_ID> -p "NewPass123!" --config /opt/prism/config.yaml

# 更新用户（例如禁用）
docker compose exec prism ./prism user update -i <USER_ID> -e false --config /opt/prism/config.yaml

# 删除用户
docker compose exec prism ./prism user delete -i <USER_ID> -f --config /opt/prism/config.yaml
```

### 环境变量与配置

`docker-compose.yml` 已内置以下变量（可按需覆盖）：

- `POSTGRES_DB`、`POSTGRES_USER`、`POSTGRES_PASSWORD`：PostgreSQL 初始化参数
- `DATABASE_URL`：Prism 数据库连接串（指向 `postgresql` 服务）
- `LOG_LEVEL`：日志级别（默认 `info`）

容器会加载 `/opt/prism/config.yaml`。请根据自己的环境修改其中的：

- `database.url`：当使用 Compose 内置数据库时，建议保持由 `DATABASE_URL` 注入；若连接外部数据库，改为外部地址（如
  `postgres://user:pass@host:5432/db?sslmode=disable`）。
- `log.level`、`log.filename`：日志级别与路径
- `app.telegram`、`app.llm`：若使用对应功能，请填入自己的凭据与模型配置

注意：不要将真实密钥提交到版本库。

### 日志与数据持久化

- 应用日志：挂载到 `prism-logs` 卷，对应容器目录 `/opt/prism/logs`
- 数据库数据：挂载到 `postgresql-data` 卷（生产 compose）或 `./data/postgresql`（开发 compose）

### 常用运维命令

```bash
# 查看运行状态
docker compose ps

# 查看应用日志
docker compose logs -f prism | cat

# 查看数据库日志
docker compose logs -f postgresql | cat

# 重启服务
docker compose restart prism

# 拉取并升级到最新镜像
docker compose pull && docker compose up -d

# 停止并删除资源（保留卷）
docker compose down

# 停止并删除资源（同时清除卷）
docker compose down -v
```

### 从源码构建镜像（可选）

仓库包含多阶段 `Dockerfile`，会先构建前端，再构建后端，最后只打包可执行文件。

```bash
# 在仓库根目录（包含 Dockerfile）
docker build -t prism:local .

# 运行（需自备可用的 PostgreSQL 地址）
docker run -d --name prism \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@host:5432/db?sslmode=disable" \
  -e LOG_LEVEL=info \
  -v $(pwd)/config.yaml:/opt/prism/config.yaml:ro \
  -v prism-logs:/opt/prism/logs \
  prism:local
```

### 仅启动本地数据库（开发场景）

若仅需本地 PostgreSQL，可使用 `docker-compose.dev.yaml`：

```bash
docker compose -f docker-compose.dev.yaml up -d
```

随后将应用的 `database.url` 指向 `localhost:5432`，或导出 `DATABASE_URL` 到运行环境。

### 端口与反向代理

- 应用默认监听 `8080`，可在反向代理（如 Nginx、Traefik）前面暴露。
- 若置于代理之后，建议在代理层添加 Gzip、TLS、访问控制等能力。

### 故障排查

- 无法连接数据库：确认 `DATABASE_URL`、数据库健康状态以及网络连通性。
- 启动后无页面：确认容器 `8080` 端口已映射且未被占用；查看 `docker compose logs -f prism`。
- 权限/配置问题：检查 `config.yaml` 是否已挂载且路径正确；必要时进入容器排查：

```bash
docker compose exec prism sh
```


