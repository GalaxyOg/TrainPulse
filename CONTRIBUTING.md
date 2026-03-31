# Contributing

Thanks for contributing to `train-notify`.

## Development Setup

```bash
python3 -m pip install -e .
```

## Test Before PR

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests -p "test_*.py" -v
```

## Pull Request Rules

- Keep changes focused and small.
- Add/adjust tests when behavior changes.
- Preserve exit-code behavior of wrapped commands.
- Do not commit real webhook URLs or secrets.
- Update `README.md` when CLI or config behavior changes.

## Commit Message Convention

Use clear prefixes:

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `test:` tests
- `chore:` maintenance

Example:

`fix: ensure FAILED event is visible in dry-run output`
