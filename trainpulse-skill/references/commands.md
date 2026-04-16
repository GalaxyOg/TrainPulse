# TrainPulse Command Reference

## Primary Commands

```bash
trainpulse run -- <command...>
trainpulse tmux-run --session <name> --log-path <path> -- <command...>
trainpulse status [--running-only] [--reconcile]
trainpulse logs [--run-id <run_id>] [--tail N] [--follow]
trainpulse stop --run-id <run_id>
trainpulse doctor
trainpulse tui
trainpulse config path
trainpulse config example
trainpulse config check
trainpulse version
```

## Typical Operation Sequences

```bash
# baseline checks
trainpulse doctor
trainpulse config check
```

```bash
# foreground run
trainpulse run -- python train.py --config cfg.yaml
```

```bash
# detached tmux run + monitoring
trainpulse tmux-run --session exp1 --log-path ./log/exp1.log -- \
  python train.py --config cfg.yaml
trainpulse status
trainpulse logs --tail 200
```

```bash
# targeted inspection + stop
trainpulse status
trainpulse logs --run-id <run_id> --follow
trainpulse stop --run-id <run_id>
```

## Config and Environment

- Config path: `~/.config/trainpulse/config.toml`
- Priority: `CLI > ENV > config.toml > defaults`
- Common env vars:
`TRAINPULSE_WEBHOOK_URL`
`TRAINPULSE_MESSAGE_TYPE`
`TRAINPULSE_STORE_PATH`
`TRAINPULSE_ERROR_LOG_PATH`
`TRAINPULSE_HEARTBEAT_MINUTES`
`TRAINPULSE_DRY_RUN`
`TRAINPULSE_REDACT`

## Troubleshooting

- `error: missing command, use: trainpulse run -- <command...>`:
Include `--` before the wrapped command.
- `error: --session is required`:
Provide `--session` for `tmux-run`.
- `error: --run-id is required`:
Read a valid id from `trainpulse status` and retry.
- Notification issues:
Run `trainpulse doctor`, then verify webhook and `dry_run` in config.
