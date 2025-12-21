# CHECK_CONDITIONS_DEBUG Environment Variable

## Overview

The `CHECK_CONDITIONS_DEBUG` environment variable enables debug mode for condition classification comparison. When set, the tool compares how conditions are classified by:
- **Legacy method**: The original hardcoded rules in `skipConditionLegacy()`
- **Config method**: The new configuration-based rules in `skipConditionViaConfig()`

## Behavior

### When CHECK_CONDITIONS_DEBUG is NOT set (default)
- The tool uses only the legacy method for classifying conditions
- No comparison warnings are shown
- Existing behavior is unchanged

### When CHECK_CONDITIONS_DEBUG is set
- The tool still uses the legacy method for making decisions
- But it also evaluates conditions using the config method
- When the two methods produce different results, a WARNING is printed
- This helps identify differences between legacy and config-based classification

## Usage

```bash
# Enable debug mode (set to any value)
export CHECK_CONDITIONS_DEBUG=1
go run github.com/guettli/check-conditions@latest all

# Or inline
CHECK_CONDITIONS_DEBUG=1 go run github.com/guettli/check-conditions@latest all
```

## Warning Messages

When differences are detected, you'll see warnings like:

```
WARNING: Legacy skips but config would NOT skip: <group> <resource> <type>=<status> <reason> "<message>"
WARNING: Legacy does NOT skip but config would skip: <group> <resource> <type>=<status> <reason> "<message>"
```

These warnings help identify conditions where the new configuration system would behave differently from the legacy system.

## Use Cases

1. **Migration validation**: Verify that your config files match legacy behavior
2. **Debug discrepancies**: Find conditions that are classified differently
3. **Config development**: Test new configuration rules against legacy behavior
