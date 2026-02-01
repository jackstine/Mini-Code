# GREP Tool Specification

## Purpose

Search for patterns in files using macOS grep.

## Tool Definition

| Field | Value |
|-------|-------|
| Name | `grep` |
| Description | Search for patterns in files or directories |

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `pattern` | string | yes | Search pattern (regex supported) |
| `path` | string | yes | File or directory path to search |
| `recursive` | boolean | no | Search recursively in directories (default: false) |

## Output Schema

**Success:**
```json
{
  "matches": "grep output as string"
}
```

**Error:**
```json
{
  "error": "error message"
}
```

## Behavior

### Pattern Matching

- Uses macOS `/usr/bin/grep`
- Supports Basic Regular Expressions (BRE) by default
- Pattern is case-sensitive

### Output Format

Matches are returned in grep's standard format:
```
filename:line_number:matching_line_content
```

For single file searches without `-r`, the format is:
```
line_number:matching_line_content
```

### Recursive Search

When `recursive` is `true`:
- Uses the `-r` flag
- Searches all files in the directory tree
- Follows the directory structure starting from `path`

### No Matches

- When no matches are found, return success with empty `matches` string
- This is not an error condition

### Error Conditions

Return an error when:
- Path does not exist
- Path is not readable (permission denied)
- Pattern is invalid (malformed regex)
