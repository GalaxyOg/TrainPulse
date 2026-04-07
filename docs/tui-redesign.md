# TrainPulse TUI 重构设计（P0/P1）

## 1. 现状评估

当前 `internal/tui/tui.go` 属于命令循环式 REPL：

- 本质是“清屏 + 文本命令输入”，不是可导航 TUI
- 缺少方向键导航、焦点模型、面板状态管理
- 缺少稳定布局（header/list/detail/help）
- 操作反馈与错误呈现能力弱
- setup/doctor/stop 等运维动作没有统一交互模型

结论：继续小修小补会让代码更难维护，且难以达成多窗格+键盘导航目标。

## 2. 架构选择

采用 Bubble Tea（状态机）+ Lip Gloss（样式）重构。

理由：

- 原生支持键盘事件、窗口 resize、tick 刷新
- `model/update/view` 分层天然适合运维控制台
- 低侵入，可保留现有 store/status/doctor/stop 语义
- Linux 终端兼容性成熟

## 3. 新架构分层

- `internal/tui/types.go`：对外 API、动作回调、消息类型
- `internal/tui/state.go`：Model 状态、过滤器、焦点、模态状态
- `internal/tui/styles.go`：颜色和样式系统
- `internal/tui/model.go`：初始化、数据刷新、统计汇总
- `internal/tui/update.go`：键盘事件与动作调度
- `internal/tui/view.go`：header/list/detail/help/modal 渲染
- `internal/tui/actions.go`：setup 写配置、doctor 执行等操作
- `internal/tui/run.go`：程序启动入口

## 4. 页面布局（文字草图）

```text
┌ Header: TrainPulse vX | now | store/config | auto-refresh | filters | counts ┐
├───────────────────────────────────────────────────────────────────────────────┤
│ Left: Runs List (selected row highlight) │ Right: Run Detail (grouped info) │
│ - status/project/job/time/duration/exit  │ - run_id/status/event             │
│ - run_id short                           │ - cmd/cwd/host/git                │
│ - scrollable                             │ - tmux/log/pid/heartbeat          │
├───────────────────────────────────────────────────────────────────────────────┤
│ Status Line: action result / error / doctor summary / attach/log tips       │
├───────────────────────────────────────────────────────────────────────────────┤
│ Help Bar: ↑↓ move | ←→ panel/filter | Tab focus | Enter apply | ...         │
└───────────────────────────────────────────────────────────────────────────────┘
```

## 5. 键位与交互模型

- `↑/↓`：在 runs 列表移动选中（focus=list）
- `←/→`：切换焦点（list/filter）或移动 filter chip
- `Tab`：循环切换焦点区
- `Enter`：在 filter 区应用筛选
- `Esc`：关闭弹层（search/confirm/setup/doctor）
- `r`：手动刷新
- `p`：自动刷新开/关
- `t`：切换最近 24h 过滤
- `/`：打开搜索输入（`p:xxx j:yyy`）
- `s`：停止当前选中 run（确认弹层）
- `a`：显示 attach 提示
- `l`：显示日志路径
- `c`：清空所有筛选（清理动作）
- `u`：打开 setup（在 TUI 内填写并保存配置）
- `d`：执行 doctor 并显示摘要
- `q`：退出

## 6. 数据刷新策略

- 初始加载一次
- 自动刷新周期默认 3s，可 `p` 暂停
- 手动刷新 `r`
- stop/setup/doctor 操作完成后触发刷新
- 保持选中 run 尽量稳定（按 run_id 回填）

## 7. 操作确认机制

- stop 操作必须二次确认（`y/n`）
- setup 保存时校验关键字段并明确结果
- doctor 结果在状态栏与弹层展示，不静默失败

## 8. 功能拆分

### P0（本次实现）

- 多窗格布局 + 颜色系统
- 方向键/Tab/Esc/Enter 可导航
- runs 列表 + 详情联动
- 手动与自动刷新
- status/filter/search/24h
- stop（确认）
- setup（TUI 内配置）
- doctor 入口与结果显示
- attach/log 提示
- clear filter 清理动作

### P1（后续增强）

- 独立日志面板（tail N / follow）
- 更多清理动作（如 notifier 错误日志清理）
- 更细粒度统计（最近失败时间、最近活动时间）
- 错误摘要提取

## 9. 与现有 CLI 的兼容性

不改变 `run/tmux-run/status/stop/logs/doctor/config` 的语义。`trainpulse tui` 仅升级交互层。
