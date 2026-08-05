# CSCAN 代码清理计划

## 📋 清理原则

1. **删除 Windows 特定代码** — 只保留 Linux/Docker 支持
2. **删除重复代码** — 合并相似功能
3. **删除废弃代码** — 移除标记为 deprecated 的代码
4. **简化 Worker** — 拆分过大的 worker.go 文件
5. **统一命名** — 确保命名一致性

## 🗑️ 已删除文件

### Windows 特定文件
- [x] `worker/loadavg_windows.go` — Windows 负载均衡（重新创建为 stub 用于编译）
- [x] `worker/restart_windows.go` — Windows 重启逻辑（重新创建为 stub 用于编译）

### 废弃代码
- [x] `worker/resource_manager.go` — 已被 AdaptiveScheduler 替代
- [x] `pkg/circuitbreaker/` — 未使用
- [x] `pkg/retry/` — 未使用
- [x] `pkg/queue/` — 未使用
- [x] `pkg/risk/` — 未使用
- [x] `pkg/logger/` — 未使用

## 📊 代码统计

| 目录 | 文件数 | 代码行数 | 状态 |
|------|--------|----------|------|
| worker/ | 25 (.go) | 7243 | ✅ 已完成拆分 |
| pkg/ | 精简 | ~4000 | ✅ 清理完成 |
| scanner/ | 15+ | 10000+ | ✅ 正常 |

## 📈 Worker 拆分结果

| 文件 | 行数 | 说明 |
|------|------|------|
| **worker.go** | 2954 | 核心逻辑（从 7107 行减少 **58.4%**） |
| worker_fingerprint_validation.go | 973 | 指纹验证任务 |
| worker_poc_validation.go | 578 | POC验证任务 |
| worker_result_save.go | 436 | 结果保存 |
| worker_auto_tag.go | 383 | 自动标签 |
| worker_heartbeat.go | 345 | 心跳/保活 |
| worker_asset_generation.go | 284 | 资产生成 |
| worker_dir_scan.go | 256 | 目录扫描 |
| worker_port_identify.go | 244 | 端口识别 |
| worker_utility.go | 224 | 工具函数 |
| worker_target_parse.go | 223 | 目标解析 |
| worker_brute_scan.go | 212 | 弱口令扫描 |
| worker_js_finder.go | 131 | JS扫描 |
| **总计** | 7243 | - |

## 🎯 目标完成情况

将 worker.go 拆分为以下文件：
1. worker.go (核心逻辑) - ✅ 完成 (2954行)
2. worker_heartbeat.go (心跳/健康监控) - ✅ 已完成
3. worker_auto_tag.go (自动标签) - ✅ 已完成
4. worker_poc_validation.go (POC验证) - ✅ 已完成
5. worker_fingerprint_validation.go (指纹验证) - ✅ 已完成
6. worker_port_identify.go (端口识别) - ✅ 已完成
7. worker_brute_scan.go (弱口令扫描) - ✅ 已完成
8. worker_dir_scan.go (目录扫描) - ✅ 已完成
9. worker_js_finder.go (JS扫描) - ✅ 已完成
10. worker_target_parse.go (目标解析) - ✅ 已完成
11. worker_result_save.go (结果保存) - ✅ 已完成
12. worker_asset_generation.go (资产生成) - ✅ 已完成
13. worker_utility.go (工具函数) - ✅ 已完成

## 📝 清理记录

| 日期 | 操作 | 说明 |
|------|------|------|
| 2026-08-05 | 拆分 worker.go | 7107行 → 2954行 (12个新文件) |
| 2026-08-05 | 删除 resource_manager.go | 废弃，已被 AdaptiveScheduler 替代 |
| 2026-08-05 | 删除 ResourceManager 引用 | Worker struct/NewWorker/applyConcurrency |
| 2026-08-05 | 删除 GetResourceStatus | adaptive_scheduler.go 废弃方法 |
| 2026-08-05 | 删除 pkg/circuitbreaker 等 | 5个未使用的包 |
| 2026-08-05 | 合并 helpers.go | buildAuthority 移入 worker_asset_generation.go |
| 2026-08-05 | 统一资源采集 | GetCPULoad/GetMemoryInfo 供调度器与心跳复用 |
| 2026-08-05 | 删除 cscan-master/cscan-executor | 孤岛原型（用户确认删除），含 cmd/master、Dockerfile.master/executor |
| 2026-08-05 | 清理 docker-compose.yaml | 移除 cscan-master/cscan-executor 服务与卷 |
| 2026-08-05 | 清理 CLAUDE.md | 移除第九章 go-zero 架构改造（v2.0.0） |
| 2026-08-05 | 清理 docs | 删除 GOZERO_ARCHITECTURE.md、IMPLEMENTATION_SUMMARY.md |
| 2026-08-05 | 清理 pkg/utils 死代码 | ip.go 15→4 函数、slice.go 删6、strings.go 20→3、utils.go 删1、blacklist.go 删 MergeMatchers |
| 2026-08-05 | 删除 task_runner.go + task_runner_integration.go | 1661行平行执行路径，主路径 executeTask 不使用 |
| 2026-08-05 | 删除 worker 死方法 | StopImmediate（被 drainAndExit 替代）、shouldStopTask |
| 2026-08-05 | 清理 NOTE 注释 | worker_heartbeat.go 3 条已过时 NOTE |
| 2026-08-05 | 运行 go mod tidy | 清理模块依赖 |
| 2026-08-05 | 运行 go vet | 无错误 |
| 2026-08-05 | 全部测试通过 | worker/model/scanner/scheduler |

## ✅ 项目验证状态

- **构建**: ✅ 成功
- **go vet**: ✅ 无错误
- **测试**: ✅ 全部通过
- **代码质量**: ✅ 符合规范