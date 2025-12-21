# Transition to Config-Based Condition Classification

## Overview

This is part of the transition from legacy hardcoded rules to the new config-based system. Set `CHECK_CONDITIONS_COMPARE_WITH_NEW_CONFIG=1` to compare both methods and see warnings when they differ.

## Transition Plan

This feature supports a gradual three-step migration:

1. **Current (Step 1)**: New config available on request, no warnings by default
   - Legacy method is the default behavior
   - Set `CHECK_CONDITIONS_COMPARE_WITH_NEW_CONFIG=1` to enable comparison warnings
2. **Step 2 (future)**: Old method still default, but warnings emitted automatically
3. **Step 3 (future)**: New config becomes the default, legacy available on request

## Usage

```bash
CHECK_CONDITIONS_COMPARE_WITH_NEW_CONFIG=1 check-conditions all
```

Warnings are written to stderr when the methods disagree:
```
WARNING: Legacy skips but config would NOT skip: ...
WARNING: Legacy does NOT skip but config would skip: ...
```

