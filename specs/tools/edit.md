# EDIT Tool Specification

## Purpose

Edit files using chunked line-based operations. Supports multiple edits in a single call for efficiency.

## Tool Definition

| Field | Value |
|-------|-------|
| Name | `edit` |
| Description | Edit a file using line-based operations (replace, insert, delete) |

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `path` | string | yes | File path to edit |
| `operations` | array | yes | List of edit operations |

### Operation Types

#### Replace

Replace a range of lines with new content.

```json
{
  "op": "replace",
  "startLine": 5,
  "endLine": 7,
  "content": ["new line 5", "new line 6"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `op` | string | `"replace"` |
| `startLine` | integer | First line to replace (1-indexed) |
| `endLine` | integer | Last line to replace (inclusive) |
| `content` | array | Replacement lines |

#### Insert

Insert new lines at a position.

```json
{
  "op": "insert",
  "afterLine": 10,
  "content": ["inserted line 1", "inserted line 2"]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `op` | string | `"insert"` |
| `afterLine` | integer | Insert after this line (0 = beginning of file) |
| `content` | array | Lines to insert |

#### Delete

Remove a range of lines.

```json
{
  "op": "delete",
  "startLine": 15,
  "endLine": 17
}
```

| Field | Type | Description |
|-------|------|-------------|
| `op` | string | `"delete"` |
| `startLine` | integer | First line to delete (1-indexed) |
| `endLine` | integer | Last line to delete (inclusive) |

## Output Schema

**Success:**
```json
{
  "path": "/absolute/path/to/file",
  "linesChanged": 5,
  "newLineCount": 120
}
```

**Error:**
```json
{
  "error": "error message"
}
```

## Behavior

### Execution Order

Operations are processed in **reverse line order** internally to preserve line numbers:

```
Input operations:
  1. Replace lines 5-7
  2. Delete lines 20-22
  3. Insert after line 50

Execution order:
  1. Insert after line 50  (highest position first)
  2. Delete lines 20-22
  3. Replace lines 5-7     (lowest position last)
```

This ensures line numbers in later operations remain valid.

### Atomic Application

- All operations succeed or none are applied
- File is written atomically (temp file + rename)
- Original file unchanged if any operation fails

### Line Indexing

- All line numbers are 1-indexed (first line = 1)
- `endLine` is inclusive
- `afterLine: 0` inserts at the beginning of the file

## Examples

### Single Replace

Replace lines 10-12 with two new lines:

```json
{
  "path": "src/main.go",
  "operations": [
    {
      "op": "replace",
      "startLine": 10,
      "endLine": 12,
      "content": ["    newFunc()", "    return nil"]
    }
  ]
}
```

### Multiple Operations

```json
{
  "path": "config.json",
  "operations": [
    {
      "op": "replace",
      "startLine": 3,
      "endLine": 3,
      "content": ["  \"port\": 9000,"]
    },
    {
      "op": "insert",
      "afterLine": 5,
      "content": ["  \"debug\": true,"]
    },
    {
      "op": "delete",
      "startLine": 10,
      "endLine": 12
    }
  ]
}
```

### Insert at Beginning

```json
{
  "path": "file.txt",
  "operations": [
    {
      "op": "insert",
      "afterLine": 0,
      "content": ["// Copyright 2024", "// MIT License", ""]
    }
  ]
}
```

### Delete Single Line

```json
{
  "path": "file.txt",
  "operations": [
    {
      "op": "delete",
      "startLine": 5,
      "endLine": 5
    }
  ]
}
```

## Error Conditions

| Condition | Error |
|-----------|-------|
| File does not exist | `"file not found: {path}"` |
| File is a directory | `"path is a directory: {path}"` |
| Permission denied | `"permission denied: {path}"` |
| Invalid line number | `"line {n} out of range (file has {total} lines)"` |
| startLine > endLine | `"invalid range: startLine {s} > endLine {e}"` |
| startLine < 1 | `"invalid line number: {n} (must be >= 1)"` |
| Empty operations array | `"no operations provided"` |
| Unknown operation type | `"unknown operation: {op}"` |
| Overlapping operations | `"operations overlap at line {n}"` |

## Overlap Detection

Operations must not affect the same lines:

```json
// ERROR: Both operations touch line 10
{
  "operations": [
    {"op": "replace", "startLine": 8, "endLine": 12, "content": ["..."]},
    {"op": "delete", "startLine": 10, "endLine": 15}
  ]
}
```

Overlap is checked before any operations are applied.

## Implementation Notes

### Processing Algorithm

```go
func applyOperations(path string, ops []Operation) error {
    content, _ := os.ReadFile(path)
    lines := strings.Split(string(content), "\n")

    // Validate all operations first
    if err := validateOperations(ops, len(lines)); err != nil {
        return err
    }

    // Sort by position descending
    sort.Slice(ops, func(i, j int) bool {
        return getPosition(ops[i]) > getPosition(ops[j])
    })

    // Apply each operation
    for _, op := range ops {
        lines = applyOperation(lines, op)
    }

    // Write atomically
    return atomicWrite(path, strings.Join(lines, "\n"))
}
```

### Operation Application

```go
func applyOperation(lines []string, op Operation) []string {
    switch op.Type {
    case "replace":
        // Remove old lines, insert new
        return splice(lines, op.StartLine-1, op.EndLine-op.StartLine+1, op.Content)

    case "insert":
        // Insert after specified line
        return splice(lines, op.AfterLine, 0, op.Content)

    case "delete":
        // Remove lines, insert nothing
        return splice(lines, op.StartLine-1, op.EndLine-op.StartLine+1, nil)
    }
    return lines
}

func splice(lines []string, start, deleteCount int, insert []string) []string {
    result := make([]string, 0, len(lines)-deleteCount+len(insert))
    result = append(result, lines[:start]...)
    result = append(result, insert...)
    result = append(result, lines[start+deleteCount:]...)
    return result
}
```

### Atomic Write

```go
func atomicWrite(path string, content string) error {
    dir := filepath.Dir(path)

    // Write to temp file
    tmp, err := os.CreateTemp(dir, ".edit-*")
    if err != nil {
        return err
    }
    tmpPath := tmp.Name()

    if _, err := tmp.WriteString(content); err != nil {
        tmp.Close()
        os.Remove(tmpPath)
        return err
    }
    tmp.Close()

    // Preserve original permissions
    if info, err := os.Stat(path); err == nil {
        os.Chmod(tmpPath, info.Mode())
    }

    // Atomic rename
    return os.Rename(tmpPath, path)
}
```

## Logging

All edit operations are logged:

**Server log (INFO):**
```
[tool] Execution started tool=edit id=toolu_123
[tool] Execution completed tool=edit id=toolu_123 duration_ms=5 success=true
```

**Agent log:**
```
=== 2024-01-15T10:30:46.010Z TOOL_CALL [edit] id=toolu_123 ===
{"path": "src/main.go", "operations": [...]}

=== 2024-01-15T10:30:46.015Z TOOL_RESULT [toolu_123] success ===
{"path": "/full/path/src/main.go", "linesChanged": 5, "newLineCount": 120}
```
