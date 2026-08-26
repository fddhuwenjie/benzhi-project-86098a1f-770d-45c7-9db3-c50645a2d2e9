# BENZHI_README

## 项目说明
- 项目：benzhi-project-86098a1f-770d-45c7-9db3-c50645a2d2e9
- 项目用途：完整实现了生物声学语料批次从建档、分层抽样、隔离双标、分歧仲裁、质量门禁到不可变发布的 HTTP 服务，并以 SQLite 同步保存聚合、幂等响应和连续审计事件。
- Go 工具链：`golang:1.23.0`
- 前端工具链：无

## 项目描述
- 项目名称：bioacoustic-corpus-release
- 项目介绍：一个面向生物声学研究团队的语料批次质控发布服务，围绕单个语料批次从建档、分层抽样、双人独立标注、分歧仲裁、质量门禁到不可变发布形成唯一闭环，并保留可核验的全过程审计记录。项目按 standard 档规划不少于 2000 行真实生产 Go 代码和不少于 20 个生产 Go 文件，根目录提供简体中文 README.md，说明用途、标准构建、运行和测试方式。
- 项目概述：一个面向生物声学研究团队的语料批次质控发布服务，围绕单个语料批次从建档、分层抽样、双人独立标注、分歧仲裁、质量门禁到不可变发布形成唯一闭环，并保留可核验的全过程审计记录。项目按 standard 档规划不少于 2000 行真实生产 Go 代码和不少于 20 个生产 Go 文件，根目录提供简体中文 README.md，说明用途、标准构建、运行和测试方式。
- 核心工作流：语料管理员创建草稿批次并登记录音片段元数据，提交后系统生成并锁定分层质控样本；两名标注员分别完成独立物种声纹标注，系统汇总一致项并把冲突项推进到待仲裁状态；仲裁专家逐项裁定后，质量负责人运行完整性、一致率和抽样覆盖门禁，门禁通过即生成带摘要的不可变发布清单，使批次依次经历草稿、待采样、双人标注中、待仲裁、待质量核验和已发布状态。
- 对外接口：提供版本化 HTTP JSON API，用于批次建档、片段登记、抽样锁定、双人标注、冲突仲裁、质量核验、发布清单读取和审计查询；服务监听地址支持 -addr=127.0.0.1:<port>，也可读取端口号形式的 PORT 并绑定 127.0.0.1:<PORT>，默认使用 127.0.0.1:19081，禁止默认绑定 8080、80、3000 或 0.0.0.0。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-86098a1f-770d-45c7-9db3-c50645a2d2e9-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-86098a1f-770d-45c7-9db3-c50645a2d2e9-arm64 linux/arm64

docker run -it benzhi-project-86098a1f-770d-45c7-9db3-c50645a2d2e9-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19081`
