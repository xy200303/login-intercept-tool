# Fenx 合规监控平台

## 本地启动

1. 复制环境变量：`Copy-Item .env.example .env`，并修改管理员密码、JWT 密钥及外部数据库凭据。
2. 启动服务：`docker compose up --build`。
3. 打开 `http://localhost:5173`，默认管理员由 `INITIAL_ADMIN_USERNAME` / `INITIAL_ADMIN_PASSWORD` 配置。

当前版本提供平台健康检查、JWT 登录（支持环境变量超级管理员及 PostgreSQL 平台账号）、代理配置、平台账号创建、监控任务创建、定时/手动运行、`kkud.user568531942` 用户采集快照、手机号优先的 `fenx_site.users` 只读关联、数据源连接测试，以及基于 PostgreSQL IP 占用记录的决策接口；冲突处理和外部账号治理将在后续迭代接入。

数据源测试接口：`POST /api/v1/connections/test`（需要 JWT）。DSN 使用根目录 `.env` 的 `KKUD_DB_DSN` 与 `FENX_DB_DSN`，建议使用只读/最小权限账号。

手动运行接口：`POST /api/v1/monitor-tasks/{id}/run`（需要 JWT）；运行记录查询：`GET /api/v1/sync-runs`。

平台账号管理：`GET/POST /api/v1/users`（仅超级管理员），创建 `agent` 角色时必须提供 `agent_id`，密码至少 8 位并使用 bcrypt 存储。
