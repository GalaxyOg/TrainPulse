---
name: trainpulse
description: Run and monitor long training jobs with TrainPulse. Use when requests mention `trainpulse run/tmux-run/status/logs/stop/tui/config/doctor`, notification-enabled training execution, or SQLite-backed run tracking and reconciliation.
---

# TrainPulse Skill

## Overview

Use this skill to run training commands with notification/state tracking and operate TrainPulse lifecycle commands on this machine.

## Workflow

1. Verify environment and configuration.
`trainpulse version`
`trainpulse doctor`
`trainpulse config check`

2. Configure once (file or TUI).
`trainpulse config path`
`trainpulse config example`
`trainpulse tui` then press `u` for setup.

3. Start jobs.
Foreground mode:
`trainpulse run -- <command...>`
Detached tmux mode:
`trainpulse tmux-run --session <name> --log-path <path> -- <command...>`

4. Monitor and inspect.
`trainpulse status`
`trainpulse logs --tail 200`
`trainpulse logs --run-id <run_id> --follow`

5. Stop or reconcile.
`trainpulse stop --run-id <run_id>`
`trainpulse status --reconcile`

## Command Guardrails

- Always include `--` between TrainPulse flags and the wrapped command.
- Never invent `run_id`; read it from `trainpulse status` first.
- For detached mode, require `--session`.
- Prefer `trainpulse doctor` before blaming webhook or tmux failures.
- Keep `status` and `logs` near `run/tmux-run` in operational workflows.

## Quick Commands

```bash
# setup and validation
trainpulse doctor
trainpulse config check

# foreground run
trainpulse run -- python train.py --config cfg.yaml

# detached tmux run
trainpulse tmux-run --session exp1 --log-path ./log/exp1.log -- \
  python train.py --config cfg.yaml

# observe and control
trainpulse status
trainpulse logs --tail 200
trainpulse stop --run-id <run_id>
```

## References

Read `references/commands.md` for command matrix, config precedence, and troubleshooting.
