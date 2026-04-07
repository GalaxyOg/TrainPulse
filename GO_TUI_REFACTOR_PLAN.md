# TrainPulse Go + TUI 重构计划（路线 A，已落地）

## 1. 当前架构简析（重构前）

重构前 TrainPulse 是 Python CLI wrapper，核心行为稳定：

- `run/tmux-run/status/stop` 子命令完整
- 配置优先级：`CLI > ENV > config.toml > 默认值`
- SQLite 持久化运行状态
- 飞书 webhook 通知（`STARTED/SUCCEEDED/FAILED/INTERRUPTED/STOPPED`）
- `HEARTBEAT` 仅本地记录，不推送
- 退出码透传

重构前问题：

- 跨服务器部署仍依赖 Python 运行时
- 二进制发布链路不完整
- TUI 运维台能力缺失

## 2. 是否建议 Go 重构

建议，且采用路线 A（全量重写）。

原因：

- 单二进制部署显著降低运维成本
- 跨机器复制与版本固定更直接
- CLI 长期维护与 release 管理更稳定
- TUI 与 CLI 共仓共语言，迭代成本更低

## 3. 原因与权衡

收益：

- 单文件可执行，减少 Python 环境漂移
- release 产物可以直接覆盖 Linux `amd64/arm64`
- 运行时行为（信号、进程组、tmux）更可控

成本：

- 首次迁移复杂度高
- 需要维护 Go 依赖（SQLite 驱动）

结论：收益覆盖成本，适合作为主线。

## 4. TUI 是否适合，建议定位

定位：**辅助运维控制台**（不是主入口）。

主入口仍保持 CLI-first；`trainpulse tui` 负责：

- 最近 runs 列表
- 过滤（状态 / 24h / project / job）
- run 详情查看
- 停止 run
- tmux attach 提示
- 日志路径查看

## 5. 推荐重构路线

采用路线 A：Go 全量重写，Python 版本停止演进（保留历史代码供参考）。

已完成内容：

- Go CLI 全量主命令实现
- 配置/运行时/通知/SQLite/tmux/TUI/doctor 完整链路
- Go 测试基线与构建链路

## 6. 分阶段实施计划（执行结果）

### Phase 1：核心域模型与存储

已完成：

- `events` 事件模型
- SQLite schema 与查询/更新能力
- 配置解析与优先级

### Phase 2：运行执行链路

已完成：

- `run/tmux-run/status/stop/logs`
- 信号转发、心跳、退出码透传
- 通知发送与重试

### Phase 3：TUI MVP

已完成：

- `tui` 交互控制台
- 列表、详情、筛选、stop、attach/log path

### Phase 4：发布与安装

已完成：

- `scripts/build_release.sh` 跨架构构建
- `scripts/install_trainpulse_binary.sh` 二进制安装脚本

## 7. 风险与兼容性问题

兼容策略：

- 配置路径保持 `~/.config/trainpulse/config.toml`
- ENV 键保持 `TRAINPULSE_*`
- 命令语义保持 `run/tmux-run/status/stop`

潜在风险：

- TUI 为轻量交互控制台，非全屏富组件框架
- `tmux-run` 依赖本机 `tmux`

## 8. 建议目录结构（Go 版）

```text
cmd/trainpulse
internal/config
internal/context
internal/events
internal/runtime
internal/store
internal/notifier
internal/tmux
internal/status
internal/logs（由 cmd 逻辑承载）
internal/doctor
internal/tui
internal/version
```

## 9. release / install 策略

release 产物建议：

- `trainpulse_<version>_linux_amd64.tar.gz`
- `trainpulse_<version>_linux_arm64.tar.gz`

产物内容：

- `trainpulse` 可执行文件
- `README.md`
- `LICENSE`

构建：

- `bash scripts/build_release.sh v0.2.0`

安装：

- `bash scripts/install_trainpulse_binary.sh v0.2.0`

---

当前结论：路线 A 已进入可用状态，后续重点应转向真实训练作业场景回归与 release 自动化（CI/CD）。
