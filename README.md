# TrainPulse

**TrainPulse** 是仓库与对外项目名，CLI 主入口统一为 `trainpulse`。

`trainpulse` 是一个训练任务通知包装器：运行任意命令并在关键事件发送飞书通知，同时保留原始退出码。

通知事件：`STARTED` / `SUCCEEDED` / `FAILED` / `INTERRUPTED`
静默运行记录事件（SQLite）：`HEARTBEAT`（仅本地记录，不推送）

时间说明：通知与运行记录默认使用 `UTC+8` 时区输出。

## ✨ 你会得到什么

- 🚀 一条命令包装训练任务：`run` / `tmux-run` / `status` / `stop`
- 📦 跨仓库项目名自动识别（优先 Git 根目录）
- 🧪 兼容 `python` / `conda run` / `uv run` / `poetry run`
- 💬 飞书 `post` 富文本 + `text` 文本通知（推荐 `post`）
- 🔒 `--redact` 脱敏、`--dry-run` 预演、SQLite 运行记录
- 🧯 通知失败不影响训练任务退出码

## 🚀 快速开始

```bash
# 1) 一次配置（长期生效，不用改 ~/.bashrc）
mkdir -p ~/.config/trainpulse
cat > ~/.config/trainpulse/config.toml <<'EOF'
[trainpulse]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/your-webhook-token"
heartbeat_minutes = 30
message_type = "post"
EOF

# 2) 直接运行（无需每次传 webhook 和检查间隔）
trainpulse run -- python train.py --config cfg.yaml
```

更多例子：

```bash
# conda
trainpulse run -- conda run -n rlzoo3 python script/run.py

# uv
trainpulse run -- uv run python train.py

# tmux 后台
trainpulse tmux-run --session exp1 --log-path ./log/train.log -- \
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
trainpulse --help
python3 -m trainpulse --help
```

安装后推荐做一次长期配置（不依赖 `bashrc`）：

```bash
mkdir -p ~/.config/trainpulse
cat > ~/.config/trainpulse/config.toml <<'EOF'
[trainpulse]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/your-webhook-token"
heartbeat_minutes = 30
message_type = "post"
store_path = "~/.local/state/trainpulse/runs.db"
EOF
```

完成后日常直接使用：

```bash
trainpulse run -- python train.py
```

## ⚙️ 配置方式与优先级

优先级始终是：

`命令行参数 > 环境变量 > ~/.config/trainpulse/config.toml > 默认值`

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
trainpulse run \
  --message-type post \
  --heartbeat-minutes 30 \
  --redact '(?i)(token=)\\S+' \
  -- python train.py
```

### 2) 环境变量

适合临时实验或 CI 临时覆盖。支持变量：

- `TRAINPULSE_WEBHOOK_URL`
- `TRAINPULSE_MESSAGE_TYPE`
- `TRAINPULSE_STORE_PATH`
- `TRAINPULSE_HEARTBEAT_MINUTES`
- `TRAINPULSE_DRY_RUN`
- `TRAINPULSE_REDACT`（逗号分隔）
- `TRAINPULSE_ERROR_LOG_PATH`

### 3) 配置文件（最低优先级）

路径：`~/.config/trainpulse/config.toml`

这是最推荐的长期配置方式。你也可以直接从仓库里的 `config.example.toml` 复制一份到本机后再改真实值；如果用环境变量方式，也可以参考 `.env.example`。

```toml
[trainpulse]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/your-webhook-token"
message_type = "post"  # text / post
store_path = "~/.local/state/trainpulse/runs.db"
heartbeat_minutes = 30
dry_run = false
redact = ["(?i)(token=)\\S+"]
```

### 4) `heartbeat-minutes` 怎么调

默认值是 **30 分钟**，用于**静默健康检查**（更新本地运行状态），不会发送“正常进行中”通知。

你可以通过三层来调：

- **CLI**：`--heartbeat-minutes`
- **ENV**：`TRAINPULSE_HEARTBEAT_MINUTES`
- **config.toml**：`heartbeat_minutes`

优先级仍然是：

`CLI > ENV > config.toml > 默认值`

例子：

```bash
# 临时把静默检查改成每 10 分钟一次
trainpulse run --heartbeat-minutes 10 -- python train.py

# 当前 shell 临时改成 20 分钟
export TRAINPULSE_HEARTBEAT_MINUTES=20
trainpulse run -- python train.py

# 长期固定成 30 分钟（推荐）
mkdir -p ~/.config/trainpulse
cat > ~/.config/trainpulse/config.toml <<'EOF'
[trainpulse]
heartbeat_minutes = 30
EOF
```

如果你完全不传，也不在环境变量或配置文件里设置，那就会使用默认的 **30**。

## 🧠 项目基本原理

可以把 `TrainPulse` / `trainpulse` 理解成一个包在训练命令外面的 wrapper：

```text
你的命令
  │
  ▼
trainpulse run / tmux-run
  │
  ├─ 解析 CLI / ENV / config.toml
  ├─ 生成 run_id / project / job / context
  ├─ 发送 STARTED
  │
  ├─ 启动原始训练命令
  │    └─ 保留并等待真实退出码
  │
  ├─ 运行中按 heartbeat_minutes 做静默检查并更新本地 HEARTBEAT
  │
  └─ 结束后根据结果发送：
       ├─ SUCCEEDED
       ├─ FAILED
       └─ INTERRUPTED

开始/结束通知 ──> notifier ──> webhook
运行记录 ──> store(SQLite)
最终退出码 ──> 原样返回给调用方
```

也就是说，它做的是：
- 帮你包住原始训练命令
- 在开始和结束节点发通知（不发送定时心跳通知）
- 记录运行状态
- 但**不吞掉原始退出码**

所以外部调度器、shell 脚本、CI 依然可以根据真实退出码判断任务成功或失败。

## 🗺️ 运行架构图

```mermaid
flowchart TD
    A[User Command<br/>trainpulse run/tmux-run -- ...] --> B[CLI Parser<br/>argparse]
    B --> C[Runtime Resolver<br/>CLI > ENV > config.toml]
    C --> D[Context Builder<br/>run_id/project/job/git/cmd]
    D --> E[CommandRunner]
    E --> F[Wrapped Training Process]
    E --> G[RunStore<br/>SQLite]
    E --> H[FeishuNotifier]
    E --> I[Silent Liveness Check]
    I --> G
    F --> E
    E --> J[Final Event<br/>SUCCEEDED/FAILED/INTERRUPTED]
    E --> H[STARTED Notification]
    J --> H[Final Notification (no HEARTBEAT)]
    J --> G
    E --> K[Exit Code passthrough]
```

## 🧪 手动测试（发布前建议）

### 成功路径

```bash
trainpulse run --dry-run -- python3 -c "print('hello')"
```

预期：看到 `STARTED` + `SUCCEEDED`，返回码 `0`

### 失败路径（重点）

```bash
trainpulse run --dry-run -- python3 -c "import sys; sys.exit(3)"
echo $?
```

预期：看到 `[trainpulse][dry-run][FAILED] ...`，返回码 `3`

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
proc=subprocess.run([sys.executable,"-m","trainpulse.cli","run","--webhook-url",url,"--",sys.executable,"-c","import sys; sys.exit(3)"])
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
- 当前版本没有可靠“卡死/假活着”判定信号，`heartbeat_minutes` 仅做静默存活检查，不会触发“疑似卡死”告警
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
