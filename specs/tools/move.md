# MOVE Tool Specification

## Purpose

Move or rename files and directories on the filesystem.

## Tool Definition

| Field | Value |
|-------|-------|
| Name | `move` |
| Description | Move or rename a file or directory |

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `source` | string | yes | Path to the file or directory to move |
| `destination` | string | yes | Target path (new location or new name) |

## Output Schema

**Success:**
```json
{
  "source": "/absolute/path/to/original",
  "destination": "/absolute/path/to/new"
}
```

**Error:**
```json
{
  "error": "error message"
}
```

## Behavior

### Move vs Rename

The operation is determined by the destination:

| Scenario | Behavior |
|----------|----------|
| Destination is new name in same directory | Rename |
| Destination is different directory | Move |
| Destination is directory that exists | Move source into that directory |

### Examples of Each

```
# Rename (same directory)
source: /project/old.txt
destination: /project/new.txt
→ File renamed to new.txt

# Move (different directory)
source: /project/file.txt
destination: /project/subdir/file.txt
→ File moved to subdir/

# Move into directory
source: /project/file.txt
destination: /project/subdir/
→ File moved to subdir/file.txt (name preserved)
```

### Directory Handling

| Source | Destination | Result |
|--------|-------------|--------|
| File | Non-existent path | File renamed/moved to destination |
| File | Existing directory | File moved into directory |
| Directory | Non-existent path | Directory renamed/moved |
| Directory | Existing directory | Source moved inside destination |

### Parent Directory Creation

- If destination parent directory doesn't exist, create it
- Use default permissions (0755) for new directories
- Create all intermediate directories as needed

### Overwrite Behavior

| Condition | Behavior |
|-----------|----------|
| Destination file exists | Overwrite it |
| Destination is non-empty directory | Error |
| Destination is empty directory | Replace it |

### Cross-Filesystem Moves

When source and destination are on different filesystems:
- Copy the file/directory to destination
- Delete the original
- Preserve permissions and timestamps

## Error Conditions

| Condition | Error |
|-----------|-------|
| Source does not exist | `"source not found: {path}"` |
| Source equals destination | `"source and destination are the same"` |
| Permission denied (source) | `"permission denied: cannot read {path}"` |
| Permission denied (destination) | `"permission denied: cannot write to {path}"` |
| Destination is non-empty directory | `"cannot overwrite non-empty directory: {path}"` |
| Moving directory into itself | `"cannot move directory into itself"` |

## Examples

### Rename File

```json
{
  "source": "old_name.go",
  "destination": "new_name.go"
}
```

Output:
```json
{
  "source": "/project/old_name.go",
  "destination": "/project/new_name.go"
}
```

### Move to Different Directory

```json
{
  "source": "src/utils.go",
  "destination": "pkg/utils/utils.go"
}
```

Output:
```json
{
  "source": "/project/src/utils.go",
  "destination": "/project/pkg/utils/utils.go"
}
```

Creates `pkg/utils/` if it doesn't exist.

### Move Into Directory

```json
{
  "source": "config.json",
  "destination": "configs/"
}
```

Output:
```json
{
  "source": "/project/config.json",
  "destination": "/project/configs/config.json"
}
```

### Rename Directory

```json
{
  "source": "src/components",
  "destination": "src/ui"
}
```

Output:
```json
{
  "source": "/project/src/components",
  "destination": "/project/src/ui"
}
```

## Implementation Notes

### Core Logic

```go
func move(source, destination string) error {
    // Resolve to absolute paths
    srcAbs, _ := filepath.Abs(source)
    dstAbs, _ := filepath.Abs(destination)

    // Check source exists
    srcInfo, err := os.Stat(srcAbs)
    if os.IsNotExist(err) {
        return fmt.Errorf("source not found: %s", source)
    }

    // If destination is existing directory, move into it
    if dstInfo, err := os.Stat(dstAbs); err == nil && dstInfo.IsDir() {
        dstAbs = filepath.Join(dstAbs, filepath.Base(srcAbs))
    }

    // Create parent directories
    if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
        return fmt.Errorf("cannot create directory: %w", err)
    }

    // Attempt rename (works for same filesystem)
    if err := os.Rename(srcAbs, dstAbs); err != nil {
        // Cross-filesystem: copy then delete
        if err := copyRecursive(srcAbs, dstAbs); err != nil {
            return err
        }
        return os.RemoveAll(srcAbs)
    }

    return nil
}
```

### Path Resolution

```go
func resolvePaths(source, dest string) (string, string, error) {
    srcAbs, err := filepath.Abs(source)
    if err != nil {
        return "", "", err
    }

    dstAbs, err := filepath.Abs(dest)
    if err != nil {
        return "", "", err
    }

    // Prevent moving into self
    if strings.HasPrefix(dstAbs, srcAbs+string(os.PathSeparator)) {
        return "", "", errors.New("cannot move directory into itself")
    }

    return srcAbs, dstAbs, nil
}
```

## Logging

**Server log (INFO):**
```
[tool] Execution started tool=move id=toolu_123
[tool] Execution completed tool=move id=toolu_123 duration_ms=2 success=true
```

**Agent log:**
```
=== 2024-01-15T10:30:46.010Z TOOL_CALL [move] id=toolu_123 ===
{"source": "old.txt", "destination": "new.txt"}

=== 2024-01-15T10:30:46.012Z TOOL_RESULT [toolu_123] success ===
{"source": "/project/old.txt", "destination": "/project/new.txt"}
```
