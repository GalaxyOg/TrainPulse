# TUI 概念与边界

## 定位

TrainPulse TUI 是运维辅助台，不是主入口。

- 主入口：CLI（可脚本化、稳定）
- 辅助入口：TUI（人工排查与操作）

## MVP 能力

- 最近 runs 列表
- 状态筛选
- 最近 24 小时筛选
- 按 project/job 过滤
- run 详情查看
- stop run
- 输出 tmux attach 命令
- 输出日志路径

## 当前交互

进入：`trainpulse tui`

常用命令：

- `setup` 初始化配置向导（可直接生成/覆盖 `~/.config/trainpulse/config.toml`）
- `h` 帮助
- `f running|failed|succeeded|interrupted|all|24h|clear24|project=<k>|job=<k>|clear`
- `d <idx|run_id>`
- `s <idx|run_id>`
- `a <idx|run_id>`
- `l <idx|run_id>`
- `q`

## 不在 MVP 范围

- 富图形组件
- 实时图表
- 复杂权限与多用户协作
