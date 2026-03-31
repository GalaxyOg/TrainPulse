# Train Notify

跨项目可复用的训练任务通知与监管工具。它可包装任意训练命令，在 `STARTED / SUCCEEDED / FAILED / INTERRUPTED` 时发送飞书 Webhook 通知，并支持 `tmux` 后台运行与多任务并发监管。

## 功能概览

- 支持任意命令包装（原生命令、`conda run ...`、`uv run ...`、`poetry run ...`、`venv` 均可）
- 自动识别项目名：优先 `git rev-parse --show-toplevel`，否则当前目录名
- 统一 CLI：
  - `train-notify run -- <command...>`
  - `train-notify tmux-run --session <name> -- <command...>`
  - `train-notify status`
  - `train-notify stop --run-id <id>`
- 飞书消息：`text` 与 `post`（富文本）格式
- 本地持久化：SQLite 记录任务状态与历史
- 信号处理：`SIGINT` / `SIGTERM` 转发子进程并发送 `INTERRUPTED`
- 长任务心跳：支持定时 `HEARTBEAT` 通知
- 安全：支持 `--redact` 命令脱敏，避免 token/密钥泄漏
- `--dry-run` 预览消息，不实际发送

## 安装

```bash
pip install .
# 或
pipx install .
```

## 运行测试

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests -p "test_*.py" -v
```

## 快速开始

1) 设置 Webhook（推荐环境变量，不写入仓库）：

```bash
export TRAIN_NOTIFY_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
```

2) 运行训练命令：

```bash
# 原生命令
train-notify run -- python train.py --config cfg.yaml

# conda
train-notify run -- conda run -n rlzoo3 python script/run.py --config-name=foo

# uv
train-notify run -- uv run python train.py
```

3) tmux 后台：

```bash
train-notify tmux-run --session ffsm_expert --log-path ./log/train.log -- \
  conda run -n rlzoo3 python -m rl_zoo3.train --algo sac --env FFSMEnv6dof-v0
```

4) 查看状态与停止：

```bash
train-notify status
train-notify stop --run-id 20260331-120001-ab12cd34
```

## 配置优先级

`CLI 参数 > 环境变量 > 配置文件 > 默认值`

- 配置文件：`~/.config/train-notify/config.toml`
- 示例：

```toml
[train_notify]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
message_type = "post"  # text / post
store_path = "~/.local/state/train-notify/runs.db"
heartbeat_minutes = 30
dry_run = false
redact = ["(?i)my_secret=\\S+"]
```

环境变量：

- `TRAIN_NOTIFY_WEBHOOK_URL`
- `TRAIN_NOTIFY_MESSAGE_TYPE`
- `TRAIN_NOTIFY_STORE_PATH`
- `TRAIN_NOTIFY_HEARTBEAT_MINUTES`
- `TRAIN_NOTIFY_DRY_RUN`
- `TRAIN_NOTIFY_REDACT`（逗号分隔）
- `TRAIN_NOTIFY_ERROR_LOG_PATH`

## 通知字段

最少字段均已覆盖：

- `event`
- `project`
- `job_name`
- `run_id`
- `host`
- `start_time` / `end_time` / `duration`
- `exit_code`
- `cmd`（脱敏）
- `log_path`
- `cwd`
- `git_branch` / `git_commit`

## 与 Python 环境工具兼容说明

`train-notify` 不接管环境本身，只包装并执行你传入的命令；因此天然兼容：

- `conda run -n <env> ...`
- `uv run ...`
- `poetry run ...`
- `venv`（激活后直接运行）

## 故障排查

- 无通知：
  - 检查 `TRAIN_NOTIFY_WEBHOOK_URL` 是否有效
  - 发送失败日志见：`~/.local/state/train-notify/notifier_errors.log`
- `tmux-run` 失败：
  - 确认系统已安装 `tmux`
  - 会话名冲突时更换 `--session`
- 命令中的敏感字段泄漏风险：
  - 使用 `--redact '<regex>'` 或 `TRAIN_NOTIFY_REDACT`

## 已知边界

- `SIGKILL`、机器断电、内核崩溃等不可捕获场景，无法即时发送中断通知。
- `tmux stop` 先发送 `Ctrl+C`，若进程不响应再强制 kill，会存在极短窗口无法优雅收尾。

## npm thin-wrapper（可选方案）

主实现保持 Python，原因：

- 训练任务生态主要在 Python
- 与 `conda/uv/venv` 对接更直接
- 部署与依赖更轻

如需 Node 入口，可额外加一个 npm 包，仅转发到本地 `train-notify` 命令。
