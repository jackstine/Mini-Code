# READ Tool Specification

## Purpose

Read the contents of a file from the filesystem.

## Tool Definition

| Field | Value |
|-------|-------|
| Name | `read` |
| Description | Read file contents, optionally specifying a line range |

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Absolute or relative file path |
| `start_line` | integer | no | First line to read (1-indexed) |
| `end_line` | integer | no | Last line to read (inclusive) |

## Output Schema

**Success:**
```json
{
  "content": "file contents as string"
}
```

**Error:**
```json
{
  "error": "error message"
}
```

## Behavior

### Line Range Handling

| start_line | end_line | Result |
|------------|----------|--------|
| omitted | omitted | Read entire file |
| provided | omitted | Read from `start_line` to end of file |
| omitted | provided | Read from line 1 to `end_line` |
| provided | provided | Read from `start_line` to `end_line` |

### Line Indexing

- Lines are 1-indexed (first line is line 1)
- `end_line` is inclusive (line at `end_line` is included in output)

### Error Conditions

Return an error when:
- File does not exist
- Path is a directory
- File is not readable (permission denied)
- `start_line` is less than 1
- `start_line` is greater than `end_line`
- `start_line` exceeds the number of lines in the file
