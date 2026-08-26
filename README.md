# 生物声学语料质控发布服务

本项目为生物声学研究团队提供单批次语料质控发布闭环。管理员登记批次和录音片段元数据后，可在草稿冻结前原子批量纠错或撤销片段，系统随后执行可复现的分层抽样；两名标注员在结果隔离条件下独立或批量提交类群标签，分歧由分类学专家仲裁；质量负责人运行完整率、一致率、分层覆盖率和未决冲突门禁。门禁通过后，服务生成排序稳定、带 SHA-256 摘要的不可变发布清单，并保留修订连续的只追加审计时间线及不可变质量重检历史。

服务只登记音频 URI 和内容摘要，不上传或保存原始音频文件。所有写命令均要求 `request_id`、`actor_id` 和 `expected_revision`；相同 `request_id` 的同一命令会跨进程重启返回原响应，不同命令复用该标识会被拒绝。

## 环境要求

- Go 1.23 或更高版本
- 可写的本地目录，用于 SQLite 数据库

项目使用纯 Go SQLite 驱动，不要求系统预装 SQLite 命令行工具或 C 编译器。

## 构建

```bash
go mod download
go build ./cmd/server
```

## 运行

默认仅监听高位回环地址 `127.0.0.1:19081`，数据库文件默认为 `bioacoustic-corpus.db`：

```bash
go run ./cmd/server
```

可显式指定监听地址和数据库路径：

```bash
go run ./cmd/server -addr=127.0.0.1:19181 -db=./corpus.db
```

未提供 `-addr` 时，也可通过端口号形式的 `PORT` 配置监听端口；服务始终绑定 `127.0.0.1`。监听地址必须是回环地址，端口须在 1024 至 65535 之间。

## 自检与测试

标准测试命令：

```bash
go test ./...
```

端到端自检会创建临时 SQLite 数据库，在真实回环 HTTP 监听器上完成建档、登记、抽样、双标隔离、分歧仲裁、质量发布、清单读取和审计校验，然后主动关闭并退出：

```bash
go run ./cmd/server -self-check -addr=127.0.0.1:19081
```

## HTTP API

API 使用 `application/json`，请求体上限为 1 MiB，并拒绝未知字段。写命令的公共元数据结构如下：

```json
{
  "meta": {
    "request_id": "request-001",
    "actor_id": "admin-user",
    "expected_revision": 0
  }
}
```

主要路由：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `GET` | `/healthz` | 存活检查 |
| `POST` | `/v1/batches` | 创建草稿批次 |
| `GET` | `/v1/batches/{batchID}` | 读取批次摘要、片段和最近质量结果 |
| `POST` | `/v1/batches/{batchID}/clips` | 批量登记片段 |
| `PATCH` | `/v1/batches/{batchID}/clips` | 在草稿状态按片段标识批量纠错，字段可部分提供 |
| `DELETE` | `/v1/batches/{batchID}/clips` | 在草稿状态按 `clip_ids` 批量撤销片段 |
| `POST` | `/v1/batches/{batchID}/submit` | 提交并冻结草稿配置 |
| `POST` | `/v1/batches/{batchID}/sample` | 按分层配额生成并锁定样本 |
| `GET/POST` | `/v1/batches/{batchID}/sample/preview` | 只读预览可复现分层抽样 |
| `POST` | `/v1/batches/{batchID}/annotations` | 提交单条标注，或通过 `annotations` 数组原子交付 1 至 500 条标注 |
| `GET` | `/v1/batches/{batchID}/clips/{clipID}/annotations?actor_id=...` | 按隔离规则读取标注 |
| `GET` | `/v1/batches/{batchID}/annotations?actor_id=...` | 查询标注员进度与待办片段 |
| `GET` | `/v1/batches/{batchID}/conflicts` | 读取待仲裁冲突 |
| `POST` | `/v1/batches/{batchID}/adjudications` | 提交分类学裁定 |
| `POST` | `/v1/batches/{batchID}/quality-check` | 运行门禁，通过时原子发布 |
| `GET` | `/v1/batches/{batchID}/quality-checks?passed=...&issue_code=...&clip_id=...&stratum=...&min_revision=...&max_revision=...&offset=0&limit=100` | 筛选分页读取质量检查历史和相邻检查对比 |
| `GET` | `/v1/batches/{batchID}/quality-checks/{sequence}` | 读取单次质量检查记录 |
| `GET` | `/v1/batches/{batchID}/manifest?offset=0&limit=100` | 分页读取并复核不可变发布清单摘要 |
| `GET` | `/v1/batches/{batchID}/audit?actor_id=...&event_type=...&offset=0&limit=100` | 筛选分页并验证审计时间线连续性 |

批次状态依次为 `draft`、`pending_sampling`、`annotating`、`pending_adjudication`、`pending_quality_review` 和 `published`。没有分歧时会跳过仲裁状态；质量门禁失败后保留问题清单，可在更新修订号后修正标注并重新检查。每次检查记录都包含稳定问题业务键，并给出相对上次检查的新增、持续、已解决问题和指标变化。发布后所有业务写入均被拒绝，但质量历史和最终对比仍可读取。
