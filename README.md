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

网络说明：
- `curl | bash` 依赖 GitHub Release 资产下载，部分网络环境会不稳定。
- 如果你在国内服务器且不能开代理，优先使用 `go install` 方式。

### 1) Go 安装（推荐，网络不稳时首选）

```bash
export GOPROXY=https://goproxy.cn,direct
export GOSUMDB=sum.golang.google.cn
GOBIN="$HOME/.local/bin" go install github.com/trainpulse/trainpulse/cmd/trainpulse@v0.2.5
```

### 2) 一键二进制安装（网络稳定时）

```bash
curl -fsSL https://raw.githubusercontent.com/GalaxyOg/TrainPulse/master/scripts/install_trainpulse_binary.sh | \
  bash -s -- v0.2.5 GalaxyOg/TrainPulse
```

### 3) 本地脚本安装二进制

```bash
# 例：安装 v0.2.5（需替换为你的 release tag）
bash scripts/install_trainpulse_binary.sh v0.2.5
```

### 4) 源码构建安装

```bash
go build -o trainpulse ./cmd/trainpulse
sudo install -m 0755 trainpulse /usr/local/bin/trainpulse
```

安装后验证：

```bash
trainpulse version
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

### 基础教程（从 0 到可用）

1) 先做环境自检

```bash
trainpulse doctor
```

2) 首次配置（推荐直接在 TUI 完成）

```bash
trainpulse tui
```

进入后按 `u` 打开 setup，依次填写：
- `webhook_url`
- `message_type`（`text` 或 `post`）
- `store_path`
- `error_log_path`
- `heartbeat_minutes`
- `dry_run`

3) 前台启动一次训练

```bash
trainpulse run -- python train.py --config cfg.yaml
```

4) 后台（tmux）启动训练

```bash
trainpulse tmux-run --session exp1 --log-path ./log/train.log -- \
  python train.py --config cfg.yaml
```

5) 查看状态与日志

```bash
trainpulse status
trainpulse logs --tail 200
```

6) 停止某个运行

```bash
trainpulse stop --run-id <run_id>
```

### 常用命令速查

```bash
trainpulse run -- <cmd...>
trainpulse tmux-run --session <name> -- <cmd...>
trainpulse status [--running-only] [--reconcile]
trainpulse stop --run-id <run_id>
trainpulse logs [--run-id <run_id>] [--tail N] [--follow]
trainpulse doctor
trainpulse tui
trainpulse config path|example|check
trainpulse version
```

### TUI 运维台

```bash
trainpulse tui
```

TUI 关键操作：

- `↑/↓` 选择 run
- `Tab` 切换焦点（列表/过滤区）
- `←/→` 切换面板或过滤 chips
- `Enter` 应用当前过滤
- `r` 手动刷新，`p` 开/关自动刷新，`t` 切换最近 24h
- `/` 搜索（输入 `p:<project> j:<job>`）
- `s` 停止选中 run（带确认）
- `a` 查看 tmux attach 命令
- `l` 打开日志弹层（tail / follow / reload / PgUp/PgDn/Home/End/j/k 滚动）
- `c` 清空过滤条件
- `x` 打开清理动作（清空错误日志 / reconcile orphaned runs）
- `u` 打开 setup 配置向导
- `d` 执行 doctor 并查看结果
- `q` 退出

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

## 发布

```bash
# 生成 linux/amd64 + linux/arm64 release 包
bash scripts/build_release.sh v0.2.5
```

产物位于 `dist/`。

## 文档

- `GO_TUI_REFACTOR_PLAN.md`
- `docs/go-migration.md`
- `docs/tui-concept.md`
