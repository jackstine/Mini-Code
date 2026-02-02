# WRITE Tool Specification

## Purpose

Write content to a file on the filesystem.

## Tool Definition

| Field | Value |
|-------|-------|
| Name | `write` |
| Description | Write content to a file, creating or overwriting as needed |

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute or relative file path |
| `content` | string | yes | Content to write to the file |
| `mode` | string | no | Write mode: `overwrite` (default) or `append` |

## Output Schema

**Success:**
```json
{
  "bytesWritten": 1234,
  "path": "/absolute/path/to/file"
}
```

**Error:**
```json
{
  "error": "error message"
}
```

## Behavior

### Write Modes

| Mode | Behavior |
|------|----------|
| `overwrite` | Replace file contents entirely (default) |
| `append` | Add content to end of existing file |

### File Creation

| Condition | Behavior |
|-----------|----------|
| File exists | Overwrite or append based on mode |
| File does not exist | Create file and parent directories |
| Parent directory missing | Create parent directories recursively |

### Directory Creation

When creating parent directories:
- Use default permissions (0755 for directories)
- Create all missing intermediate directories
- Fail if any path component exists as a file

### File Permissions

| Item | Permission |
|------|------------|
| New files | 0644 (rw-r--r--) |
| New directories | 0755 (rwxr-xr-x) |
| Existing files | Permissions preserved |

### Content Handling

- Content is written as UTF-8 encoded text
- No automatic newline added at end of content
- Binary content not supported (use base64 encoding if needed)

### Atomicity

- Write to temporary file first, then rename
- Ensures file is never left in partial state
- Temporary file in same directory as target

## Error Conditions

Return an error when:
- Path is empty
- Path is a directory
- Permission denied (file or parent directory)
- Disk full
- Path component exists as file (when creating directories)
- Invalid path (null bytes, etc.)

## Examples

### Create New File

Input:
```json
{
  "path": "config.json",
  "content": "{\n  \"port\": 8080\n}"
}
```

Output:
```json
{
  "bytesWritten": 20,
  "path": "/home/user/project/config.json"
}
```

### Append to File

Input:
```json
{
  "path": "log.txt",
  "content": "New log entry\n",
  "mode": "append"
}
```

Output:
```json
{
  "bytesWritten": 14,
  "path": "/home/user/project/log.txt"
}
```

### Create with Nested Directories

Input:
```json
{
  "path": "src/components/Button.tsx",
  "content": "export function Button() { return <button>Click</button> }"
}
```

Output:
```json
{
  "bytesWritten": 58,
  "path": "/home/user/project/src/components/Button.tsx"
}
```

Creates `src/` and `src/components/` directories if they don't exist.

## Security Considerations

### Path Validation

- Resolve relative paths against harness working directory
- No path traversal restrictions (agent has full filesystem access)
- Symlinks are followed

### Logging

All write operations are logged:
- File path
- Bytes written
- Write mode
- Success/failure status
