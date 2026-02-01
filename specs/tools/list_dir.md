# LIST_DIR Tool Specification

## Purpose

List files and directories at a given path using `ls -al` output format.

## Tool Definition

| Field | Value |
|-------|-------|
| Name | `list_dir` |
| Description | List directory contents with detailed metadata |

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | Directory path to list |

## Output Schema

**Success:**
```json
{
  "entries": "raw ls -al output as string"
}
```

**Error:**
```json
{
  "error": "error message"
}
```

## Behavior

### Command Execution

- Executes `ls -al <path>`
- Returns the raw output as a string

### Output Contents

The output includes:
- Hidden files (files starting with `.`)
- File permissions
- Owner and group
- File size in bytes
- Modification date
- File/directory name

### Error Conditions

Return an error when:
- Path does not exist
- Path is not a directory
- Directory is not readable (permission denied)
