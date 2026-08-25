# BENZHI 评测说明

基于 Go 实现的密码硬件随机数健康度追溯后端服务，一款后端服务，完成采样窗口序列校验、熵估计与 NIST 风格健康测试判定、跨窗口重复重播检测、瞬态与持续异常关联归因、熵源隔离降级与不可变诊断快照封存。

## 启动

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/rnghealth --addr :8080 --db rnghealth.db
```

## 自检（不启动长驻服务）

```bash
go run ./cmd/rnghealth --smoke-test
```

`--smoke-test` 会真实创建熵源与采样窗口、执行健康测试、触发持续异常关联、记录重启边界、发布诊断快照，关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出码结束。

## 构建门禁

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go build ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go vet   ./...
CGO_ENABLED=0 GOTOOLCHAIN=local go test  ./...
go run ./cmd/rnghealth --smoke-test
```

## HTTP API（前缀 /api）

熵源：`POST /api/entropy-sources`、`GET /api/entropy-sources`、`GET /api/entropy-sources/{id}`、`POST /api/entropy-sources/{id}/degrade`、`.../isolate`、`.../seal`、`.../recover`、`.../restart`
窗口：`POST /api/windows`、`POST /api/windows/batch`、`GET /api/windows`、`GET /api/windows/{id}`、`GET /api/windows/{id}/tests`
健康：`POST /api/health/reevaluate`、`GET /api/health/categories`
事件：`GET /api/events`、`GET /api/events/{id}`、`POST /api/events/{id}/confirm`、`POST /api/events/{id}/close`
分析：`GET /api/sources/{id}/analysis`、`GET /api/stats`
快照：`POST /api/snapshots`、`GET /api/snapshots`、`GET /api/snapshots/{id}`、`POST /api/snapshots/{id}/publish`、`POST /api/snapshots/{id}/supersede`
自检：`GET /api/selfcheck`

## 持久化

SQLite（modernc.org/sqlite，CGO 无关）。建表：entropy_sources、sample_windows、health_tests、health_events、restart_boundaries、diagnostic_snapshots。窗口序号幂等；封存快照不改变既有统计证据。
