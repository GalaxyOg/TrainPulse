# Release Checklist

This project is not published automatically yet. Use this checklist for manual releases.

## 1) Pre-check

- Confirm `README.md`, `LICENSE`, `CONTRIBUTING.md` are up to date.
- Run full tests:

```bash
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover -s tests -p "test_*.py" -v
```

- Verify local install:

```bash
python3 -m pip install -e .
trainpulse --help
```

## 2) Version bump

- Update version in `pyproject.toml` and `trainpulse/__init__.py`.
- Add release notes (recommended in GitHub Release page).

## 3) Build artifacts

```bash
python3 -m pip install --upgrade build
python3 -m build
```

Expected outputs in `dist/`:

- `.tar.gz` source distribution
- `.whl` wheel

## 4) Publish (when registry is ready)

```bash
python3 -m pip install --upgrade twine
python3 -m twine upload dist/*
```

## 5) Post-release checks

- Install from published package in a clean env.
- Run one success and one failed command smoke test.
- Verify webhook message format in Feishu.
