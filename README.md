# Train Notify

`train-notify` 是一个训练任务通知包装器：运行任意命令并在关键事件发送飞书通知，同时保留原始退出码。

支持事件：`STARTED` / `SUCCEEDED` / `FAILED` / `INTERRUPTED` / `HEARTBEAT`

## ✨ 你会得到什么

- 🚀 一条命令包装训练任务：`run` / `tmux-run` / `status` / `stop`
- 📦 跨仓库项目名自动识别（优先 Git 根目录）
- 🧪 兼容 `python` / `conda run` / `uv run` / `poetry run`
- 💬 飞书 `post` 富文本 + `text` 文本通知（推荐 `post`）
- 🔒 `--redact` 脱敏、`--dry-run` 预演、SQLite 运行记录
- 🧯 通知失败不影响训练任务退出码

## 🚀 快速开始

```bash
export TRAIN_NOTIFY_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
train-notify run -- python train.py --config cfg.yaml
```

更多例子：

```bash
# conda
train-notify run -- conda run -n rlzoo3 python script/run.py

# uv
train-notify run -- uv run python train.py

# tmux 后台
train-notify tmux-run --session exp1 --log-path ./log/train.log -- \
  python train.py --config cfg.yaml
```

## 📦 本地安装

要求：Python 3.10+

```bash
python3 -m pip install -e .
# 或
pipx install .
```

验证：

```bash
train-notify --help
python3 -m train_notify --help
```

## ⚙️ 配置方式与优先级

优先级始终是：

`命令行参数 > 环境变量 > ~/.config/train-notify/config.toml > 默认值`

### 1) 命令行参数（最高优先级）

`run` / `tmux-run` 常用参数：

- `--webhook-url`
- `--message-type text|post`
- `--store-path`
- `--error-log-path`
- `--heartbeat-minutes`
- `--dry-run / --no-dry-run`
- `--redact <regex>`（可重复）
- `--job-name`
- `--log-path`

示例：

```bash
train-notify run \
  --message-type post \
  --heartbeat-minutes 30 \
  --redact '(?i)(token=)\\S+' \
  -- python train.py
```

### 2) 环境变量

支持变量：

- `TRAIN_NOTIFY_WEBHOOK_URL`
- `TRAIN_NOTIFY_MESSAGE_TYPE`
- `TRAIN_NOTIFY_STORE_PATH`
- `TRAIN_NOTIFY_HEARTBEAT_MINUTES`
- `TRAIN_NOTIFY_DRY_RUN`
- `TRAIN_NOTIFY_REDACT`（逗号分隔）
- `TRAIN_NOTIFY_ERROR_LOG_PATH`

#### 临时配置（当前 shell 会话）

```bash
export TRAIN_NOTIFY_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
export TRAIN_NOTIFY_MESSAGE_TYPE="post"
```

#### 持久化配置（长期生效）

```bash
cat >> ~/.bashrc <<'EOF'
export TRAIN_NOTIFY_WEBHOOK_URL="https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
export TRAIN_NOTIFY_MESSAGE_TYPE="post"
EOF
source ~/.bashrc
```

### 3) 配置文件（最低优先级）

路径：`~/.config/train-notify/config.toml`

```toml
[train_notify]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/xxxx"
message_type = "post"  # text / post
store_path = "~/.local/state/train-notify/runs.db"
heartbeat_minutes = 30
dry_run = false
redact = ["(?i)(token=)\\S+"]
```

## 🧪 手动测试（发布前建议）

### 成功路径

```bash
train-notify run --dry-run -- python3 -c "print('hello')"
```

预期：看到 `STARTED` + `SUCCEEDED`，返回码 `0`

### 失败路径（重点）

```bash
train-notify run --dry-run -- python3 -c "import sys; sys.exit(3)"
echo $?
```

预期：看到 `[train-notify][dry-run][FAILED] ...`，返回码 `3`

### 真实 webhook 路径（本地 mock）

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
    print(e["content"].get("text") or e["content"]["post"]["zh_cn"]["title"])
PY
```

预期：返回码 `3`，收到 `STARTED` + `FAILED`

## 💬 推荐消息示例

`text`：

```text
❌ [FAILED] Task Failed | TrainPulse
🧩 job: train
🆔 run_id: 20260331-090220-6b71178b
📉 exit_code: 3
⏱️ duration: 0.204s
💻 cmd: python3 -c 'import sys; sys.exit(3)'
```

`post` 标题：

```text
❌ Task Failed · TrainPulse
```

## 🛡️ 边界与安全

- `SIGKILL` / 断电 / 宿主机崩溃不可捕获，无法即时发送中断通知
- 不要把真实 webhook 提交进仓库，优先使用 ENV 或本机配置
- 通知发送失败会记录本地日志，但不会改变训练命令退出码

## 🧰 开发与测试

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests -p "test_*.py" -v
```

## 🤝 开源协作

- License: MIT（`LICENSE`）
- 贡献指南：`CONTRIBUTING.md`
- 发布清单：`RELEASE.md`
- 安全策略：`SECURITY.md`
- 变更记录：`CHANGELOG.md`
