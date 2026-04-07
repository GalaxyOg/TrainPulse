# TrainPulse (Go)

TrainPulse 是训练任务通知与运行状态管理工具。

核心定位：包装任意训练命令，在关键事件发送通知，持久化运行状态，并保留原始退出码。

## 特性

- CLI-first：`run / tmux-run / status / stop / logs / doctor / tui / config`
- 飞书 webhook 通知：`STARTED / SUCCEEDED / FAILED / INTERRUPTED / STOPPED`
- `HEARTBEAT` 只写本地 SQLite，不推送
- SQLite 本地状态库
- tmux 后台运行
- 退出码透传

## 安装

### 1) 一键安装（推荐）

```bash
curl -fsSL https://raw.githubusercontent.com/GalaxyOg/TrainPulse/master/scripts/install_trainpulse_binary.sh | \
  bash -s -- v0.2.1 GalaxyOg/TrainPulse
```

### 2) 下载二进制脚本安装

```bash
# 例：安装 v0.2.1（需替换为你的 release tag）
bash scripts/install_trainpulse_binary.sh v0.2.1
```

### 3) 本地构建

```bash
go build -o trainpulse ./cmd/trainpulse
sudo install -m 0755 trainpulse /usr/local/bin/trainpulse
```

## 快速开始

### 配置文件

默认路径：`~/.config/trainpulse/config.toml`

```toml
[trainpulse]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/your-webhook-token"
message_type = "post"
store_path = "~/.local/state/trainpulse/runs.db"
error_log_path = "~/.local/state/trainpulse/notifier_errors.log"
heartbeat_minutes = 30
dry_run = false
redact = ["(?i)(token=)\\S+"]
```

### 运行训练

```bash
trainpulse run -- python train.py --config cfg.yaml
```

### tmux 后台运行

```bash
trainpulse tmux-run --session exp1 --log-path ./log/train.log -- \
  python train.py --config cfg.yaml
```

### 状态查看与停止

```bash
trainpulse status
trainpulse stop --run-id <run_id>
```

### 日志

```bash
trainpulse logs --run-id <run_id> --tail 200
trainpulse logs --run-id <run_id> --follow
```

### 环境自检

```bash
trainpulse doctor
```

### TUI 运维台

```bash
trainpulse tui
```

首次安装后可直接在 TUI 内完成配置：

1. 启动 `trainpulse tui`
2. 输入 `setup`
3. 按提示填写 `webhook_url/message_type/store_path/error_log_path/heartbeat_minutes/dry_run`
4. 完成后执行 `trainpulse doctor`
5. 然后执行 `trainpulse run -- <cmd...>`

## 配置优先级

始终为：

`CLI > ENV > config.toml > 默认值`

支持环境变量：

- `TRAINPULSE_WEBHOOK_URL`
- `TRAINPULSE_MESSAGE_TYPE`
- `TRAINPULSE_STORE_PATH`
- `TRAINPULSE_ERROR_LOG_PATH`
- `TRAINPULSE_HEARTBEAT_MINUTES`
- `TRAINPULSE_DRY_RUN`
- `TRAINPULSE_REDACT`

## 命令概览

```bash
trainpulse run -- <cmd...>
trainpulse tmux-run --session <name> -- <cmd...>
trainpulse status [--running-only] [--reconcile]
trainpulse stop --run-id <run_id>
trainpulse logs [--run-id <run_id>] [--tail N] [--follow]
trainpulse doctor
trainpulse tui
# 在 TUI 内输入 setup 可完成初始化配置
trainpulse config path|example|check
trainpulse version
```

## 发布

```bash
# 生成 linux/amd64 + linux/arm64 release 包
bash scripts/build_release.sh v0.2.1
```

产物位于 `dist/`。

## 文档

- `GO_TUI_REFACTOR_PLAN.md`
- `docs/go-migration.md`
- `docs/tui-concept.md`
