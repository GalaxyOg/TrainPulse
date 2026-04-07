# TUI 概念与边界

## 定位

TrainPulse TUI 是本地训练任务运维控制台，不是替代 CLI 的唯一入口。

- 主入口：CLI（可脚本化、可自动化）
- 值守入口：TUI（人工观察、排障、运维操作）

## 布局模型

- Header：版本、时间、store/config 摘要、刷新状态、过滤摘要、状态统计
- Left / Runs List：可滚动任务列表，当前选中行高亮
- Right / Run Detail：展示选中 run 的完整信息分组
- Status Line：动作结果与错误反馈
- Help Bar：固定显示快捷键

## 交互模型

进入：`trainpulse tui`

核心键位：

- `↑/↓` 列表选中移动
- `Tab` 切换焦点（列表/过滤）
- `←/→` 切换面板或过滤 chips
- `Enter` 在过滤区应用筛选
- `Esc` 关闭弹层
- `r` 手动刷新
- `p` 开/关自动刷新
- `t` 切换最近 24h 过滤
- `/` 搜索（`p:<project> j:<job>`）
- `s` 停止选中 run（确认后执行）
- `a` 查看 attach 命令
- `l` 打开日志弹层（tail / follow / reload，支持 PgUp/PgDn/Home/End/j/k 滚动）
- `c` 清空过滤条件
- `x` 打开清理动作（clear filters / clear notifier error log / reconcile orphaned runs）
- `u` 打开 setup 向导（TUI 内直接写配置）
- `d` 运行 doctor 并展示结果
- `q` 退出

## 当前范围（P0 + P1 已落地）

- 多窗格导航
- 状态高亮
- 列表/详情联动
- 搜索和筛选
- stop/setup/doctor 动作入口
- attach 提示
- 日志 tail/follow 弹层
- 清理动作弹层
- 统计增强（last failed / last active）
- 错误摘要展示（基于日志尾部提取）
