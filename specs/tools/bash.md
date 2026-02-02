# BASH Tool Specification

## Purpose

Execute bash commands and return the output.

## Tool Definition

| Field | Value |
|-------|-------|
| Name | `bash` |
| Description | Execute a bash command and return stdout/stderr |

## Input Schema

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | The bash command to execute |

## Output Schema

**Success:**
```json
{
  "stdout": "standard output as string",
  "stderr": "standard error as string",
  "exitCode": 0
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

- Execute command using `/bin/bash -c "<command>"`
- Capture both stdout and stderr separately
- Wait for command to complete before returning
- Return exit code with output

### Timeout

| Setting | Value |
|---------|-------|
| Default timeout | 30 seconds |
| Behavior on timeout | Kill process, return timeout error |

### Working Directory

- Commands execute in the harness working directory
- No directory isolation (commands can access filesystem)

### Environment

- Inherits harness process environment variables
- No additional environment variables injected

### Output Handling

| Condition | Behavior |
|-----------|----------|
| Command succeeds | Return stdout, stderr, exitCode: 0 |
| Command fails | Return stdout, stderr, actual exitCode |
| Command not found | Return error with message |
| Permission denied | Return error with message |

### Output Limits

| Limit | Value |
|-------|-------|
| Max stdout size | 1 MB |
| Max stderr size | 1 MB |
| Truncation | Truncate with "... (truncated)" suffix |

## Security Considerations

### Allowed Operations

The bash tool executes commands with the same permissions as the harness process. The agent can:
- Read and write files
- Execute programs
- Access network
- Modify system state

### Logging

All bash commands are logged to the agent interaction log:
- Command string
- Exit code
- Output size (not full content in server logs)

## Error Conditions

Return an error when:
- Command is empty string
- Timeout exceeded
- Process spawn fails
- System resources unavailable

## Examples

### Simple Command

Input:
```json
{
  "command": "echo 'hello world'"
}
```

Output:
```json
{
  "stdout": "hello world\n",
  "stderr": "",
  "exitCode": 0
}
```

### Command with Error

Input:
```json
{
  "command": "ls /nonexistent"
}
```

Output:
```json
{
  "stdout": "",
  "stderr": "ls: /nonexistent: No such file or directory\n",
  "exitCode": 1
}
```

### Piped Command

Input:
```json
{
  "command": "cat file.txt | grep pattern | wc -l"
}
```

Output:
```json
{
  "stdout": "42\n",
  "stderr": "",
  "exitCode": 0
}
```
