# Transition to Config-Based Condition Classification

## Overview

This tool supports a gradual transition from legacy hardcoded rules to a new config-based system for condition classification. You can control which method is used via the `--mode` flag or `CHECK_CONDITIONS_MODE` environment variable.

## Modes

### 1. `only-old` (default)
Uses only the legacy hardcoded logic. This is the current default behavior.

```bash
check-conditions all --mode only-old
# or
CHECK_CONDITIONS_MODE=only-old check-conditions all
```

### 2. `only-new`
Uses only the new config-based logic. The legacy method is not used.

```bash
check-conditions all --mode only-new
# or
CHECK_CONDITIONS_MODE=only-new check-conditions all
```

### 3. `old-compare-new`
Uses the legacy method for decisions, but compares with the new config method and warns (to stderr) when they differ. Useful for validating that your config matches legacy behavior.

```bash
check-conditions all --mode old-compare-new
# or
CHECK_CONDITIONS_MODE=old-compare-new check-conditions all
```

### 4. `new-compare-old`
Uses the new config method for decisions, but compares with the legacy method and warns (to stderr) when they differ. Useful for testing the new config before fully switching.

```bash
check-conditions all --mode new-compare-old
# or
CHECK_CONDITIONS_MODE=new-compare-old check-conditions all
```

## Warning Messages

When in compare modes (`old-compare-new` or `new-compare-old`), warnings are written to stderr when the two methods disagree:

```
WARNING: Legacy skips but config would NOT skip: core pods Ready=False PodCompleted ""
WARNING: Config does NOT skip but legacy would skip: apps deployments Available=True ...
```

## Transition Plan

The planned migration has three steps:

1. **Step 1 (Current)**: Default is `only-old`, new config available via `only-new` or `new-compare-old`
2. **Step 2 (Future)**: Default changes to automatically compare (like `old-compare-new`), warnings emitted
3. **Step 3 (Future)**: Default changes to `only-new`, legacy available via `only-old`

## Backward Compatibility

For backward compatibility, the legacy `CHECK_CONDITIONS_COMPARE_WITH_NEW_CONFIG` environment variable is still supported and will be interpreted as `--mode old-compare-new`.

