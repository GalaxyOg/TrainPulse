# Go 迁移说明

## 目标

将 TrainPulse 主线从 Python CLI 迁移为 Go 单二进制工具，同时保持原有使用语义。

## 保持兼容的部分

- 命令风格：`run/tmux-run/status/stop`
- 配置层级：`CLI > ENV > config.toml > 默认值`
- 配置路径：`~/.config/trainpulse/config.toml`
- ENV 前缀：`TRAINPULSE_*`
- 事件语义：`STARTED/SUCCEEDED/FAILED/INTERRUPTED/STOPPED/HEARTBEAT`
- `HEARTBEAT` 仅本地记录，不推送
- 退出码透传

## 新增能力

- `doctor` 环境检查
- `logs` 日志读取与 follow
- `tui` 轻量运维控制台
- TUI 内 `setup` 初始化配置向导
- 二进制 release/install 脚本

## 建议迁移步骤

1. 使用 Go 二进制替换旧入口
2. 沿用原配置文件和环境变量
3. 用 `doctor` 检查部署环境
4. 逐步切换 `run/tmux-run` 工作流
5. 使用 `tui` 作为运维辅助，不替代 CLI
