# Train Notify

`train-notify` 是一个通用训练任务通知工具：包装任意命令，在任务 `STARTED / SUCCEEDED / FAILED / INTERRUPTED` 时发送飞书 Webhook 通知，并保留原命令退出码。

适用于深度学习/强化学习训练，不耦合具体框架。

## 核心能力

- 跨仓库自动识别项目名（优先 Git 根目录名，否则当前目录名）
- 统一命令入口：`run / tmux-run / status / stop`
- 支持任意执行方式：原生命令、`conda run ...`、`uv run ...`、`poetry run ...`
- 通知格式：`text` + `post`
- 失败不吞退出码：任务失败返回码原样透传
- 支持 `--dry-run`、`--redact`、心跳通知、SQLite 运行记录

## 快速开始

```bash
export TRAIN_NOTIFY_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
train-notify run -- python train.py --config cfg.yaml
```

常见场景：

```bash
# conda
train-notify run -- conda run -n rlzoo3 python script/run.py

# uv
train-notify run -- uv run python train.py

# tmux 后台
train-notify tmux-run --session exp1 --log-path ./log/train.log -- \
  python train.py --config cfg.yaml
```

## 本地安装

要求：Python 3.10+。

```bash
# 在仓库根目录
python3 -m pip install -e .
# 或隔离安装
pipx install .
```

安装后验证：

```bash
train-notify --help
```

## 手动测试（建议发布前跑一遍）

### 1) 成功路径

```bash
train-notify run --dry-run -- python3 -c "print('hello')"
```

预期：

- 终端出现 `[STARTED]` 和 `[SUCCEEDED]` dry-run 输出
- 命令返回码为 `0`

### 2) 失败路径（重点）

```bash
train-notify run --dry-run -- python3 -c "import sys; sys.exit(3)"
echo $?
```

预期：

- 终端出现 `[train-notify][dry-run][FAILED] ... exit=3`
- 最终返回码是 `3`

### 3) 真实 webhook 路径（本地 mock）

```bash
python3 - <<'PY'
import json, threading, subprocess, sys
from http.server import BaseHTTPRequestHandler, HTTPServer

class H(BaseHTTPRequestHandler):
    events=[]
    def do_POST(self):
        n=int(self.headers.get("Content-Length","0"))
        H.events.append(json.loads(self.rfile.read(n).decode()))
        resp=b'{"code":0,"msg":"ok"}'
        self.send_response(200); self.send_header("Content-Type","application/json")
        self.send_header("Content-Length",str(len(resp))); self.end_headers(); self.wfile.write(resp)
    def log_message(self, *args): pass

s=HTTPServer(("127.0.0.1",0),H)
t=threading.Thread(target=s.serve_forever,daemon=True); t.start()
url=f"http://127.0.0.1:{s.server_address[1]}/hook"
proc=subprocess.run([sys.executable,"-m","train_notify.cli","run","--webhook-url",url,"--",sys.executable,"-c","import sys; sys.exit(3)"])
s.shutdown(); t.join(timeout=3)
print("returncode=",proc.returncode)
print("events=",len(H.events))
for e in H.events:
    print(e["content"]["text"])
PY
```

预期：

- `returncode= 3`
- 收到 2 条事件：`STARTED` 和 `FAILED`

## CLI 概览

- `train-notify run -- <command...>`
- `train-notify tmux-run --session <name> -- <command...>`
- `train-notify status`
- `train-notify stop --run-id <id>`

## 配置

优先级：`CLI > ENV > ~/.config/train-notify/config.toml > 默认值`

示例：

```toml
[train_notify]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
message_type = "text"  # text / post
store_path = "~/.local/state/train-notify/runs.db"
heartbeat_minutes = 30
dry_run = false
redact = ["(?i)(token=)\\S+"]
```

环境变量：

- `TRAIN_NOTIFY_WEBHOOK_URL`
- `TRAIN_NOTIFY_MESSAGE_TYPE`
- `TRAIN_NOTIFY_STORE_PATH`
- `TRAIN_NOTIFY_HEARTBEAT_MINUTES`
- `TRAIN_NOTIFY_DRY_RUN`
- `TRAIN_NOTIFY_REDACT`
- `TRAIN_NOTIFY_ERROR_LOG_PATH`

## 开发与测试

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests -p "test_*.py" -v
```

## 边界与安全说明

- `SIGKILL` / 断电 / 宿主机崩溃不可捕获，无法即时发送中断通知。
- 不建议将 webhook 写入仓库文件，请优先使用环境变量或本机配置文件。
- 通知发送失败不会改变训练命令退出码；失败详情写入本地错误日志。

## 开源协作

- License: MIT（见 `LICENSE`）
- 贡献方式：见 `CONTRIBUTING.md`
- 发布建议：见 `RELEASE.md`
- 安全策略：见 `SECURITY.md`
- 变更记录：见 `CHANGELOG.md`
