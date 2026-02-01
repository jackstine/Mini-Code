import { For, type JSX } from "solid-js"

/**
 * Markdown rendering utilities for the TUI.
 * Converts markdown syntax to OpenTUI JSX elements.
 */

/**
 * Truncate text to a maximum number of lines.
 * Returns the truncated text and the number of lines removed.
 */
export function truncateLines(text: string, maxLines: number): { text: string; truncated: number } {
  const lines = text.split("\n")
  if (lines.length <= maxLines) {
    return { text, truncated: 0 }
  }
  return {
    text: lines.slice(0, maxLines).join("\n"),
    truncated: lines.length - maxLines
  }
}

/**
 * Token types for markdown parsing.
 */
export type Token =
  | { type: "text"; content: string }
  | { type: "bold"; content: string }
  | { type: "italic"; content: string }
  | { type: "code"; content: string }
  | { type: "codeblock"; content: string; language?: string }
  | { type: "heading"; level: number; content: string }
  | { type: "bullet"; content: string }
  | { type: "numbered"; number: number; content: string }
  | { type: "link"; text: string; url: string }
  | { type: "newline" }

/**
 * Parse inline markdown (bold, italic, code, links) within a line.
 * Returns an array of tokens.
 */
export function parseInline(text: string): Token[] {
  const tokens: Token[] = []
  let remaining = text

  while (remaining.length > 0) {
    // Bold: **text** or __text__
    const boldMatch = remaining.match(/^\*\*(.+?)\*\*|^__(.+?)__/)
    if (boldMatch) {
      tokens.push({ type: "bold", content: boldMatch[1] || boldMatch[2] })
      remaining = remaining.slice(boldMatch[0].length)
      continue
    }

    // Italic: *text* or _text_ (but not inside words for _)
    const italicMatch = remaining.match(/^\*([^*]+?)\*|^_([^_]+?)_(?![a-zA-Z0-9])/)
    if (italicMatch) {
      tokens.push({ type: "italic", content: italicMatch[1] || italicMatch[2] })
      remaining = remaining.slice(italicMatch[0].length)
      continue
    }

    // Inline code: `code`
    const codeMatch = remaining.match(/^`([^`]+?)`/)
    if (codeMatch) {
      tokens.push({ type: "code", content: codeMatch[1] })
      remaining = remaining.slice(codeMatch[0].length)
      continue
    }

    // Link: [text](url)
    const linkMatch = remaining.match(/^\[([^\]]+?)\]\(([^)]+?)\)/)
    if (linkMatch) {
      tokens.push({ type: "link", text: linkMatch[1], url: linkMatch[2] })
      remaining = remaining.slice(linkMatch[0].length)
      continue
    }

    // Plain text until next special character or end
    const textMatch = remaining.match(/^[^*_`\[\n]+/)
    if (textMatch) {
      tokens.push({ type: "text", content: textMatch[0] })
      remaining = remaining.slice(textMatch[0].length)
      continue
    }

    // Single special character that didn't match a pattern
    tokens.push({ type: "text", content: remaining[0] })
    remaining = remaining.slice(1)
  }

  return tokens
}

/**
 * Parse markdown text into tokens.
 * Handles block-level elements (headings, lists, code blocks) and inline formatting.
 */
export function parseMarkdown(text: string): Token[] {
  const tokens: Token[] = []
  const lines = text.split("\n")
  let i = 0

  while (i < lines.length) {
    const line = lines[i]

    // Code block: ```
    if (line.startsWith("```")) {
      const language = line.slice(3).trim() || undefined
      const codeLines: string[] = []
      i++
      while (i < lines.length && !lines[i].startsWith("```")) {
        codeLines.push(lines[i])
        i++
      }
      tokens.push({ type: "codeblock", content: codeLines.join("\n"), language })
      i++ // Skip closing ```
      if (i < lines.length) {
        tokens.push({ type: "newline" })
      }
      continue
    }

    // Heading: # ## ### etc.
    const headingMatch = line.match(/^(#{1,6})\s+(.+)$/)
    if (headingMatch) {
      tokens.push({ type: "heading", level: headingMatch[1].length, content: headingMatch[2] })
      i++
      if (i < lines.length) {
        tokens.push({ type: "newline" })
      }
      continue
    }

    // Bullet list: - item or * item
    const bulletMatch = line.match(/^[\s]*[-*]\s+(.+)$/)
    if (bulletMatch) {
      tokens.push({ type: "bullet", content: bulletMatch[1] })
      i++
      if (i < lines.length) {
        tokens.push({ type: "newline" })
      }
      continue
    }

    // Numbered list: 1. item
    const numberedMatch = line.match(/^[\s]*(\d+)\.\s+(.+)$/)
    if (numberedMatch) {
      tokens.push({ type: "numbered", number: parseInt(numberedMatch[1]), content: numberedMatch[2] })
      i++
      if (i < lines.length) {
        tokens.push({ type: "newline" })
      }
      continue
    }

    // Empty line
    if (line.trim() === "") {
      tokens.push({ type: "newline" })
      i++
      continue
    }

    // Regular paragraph - parse inline elements
    const inlineTokens = parseInline(line)
    tokens.push(...inlineTokens)
    i++
    if (i < lines.length) {
      tokens.push({ type: "newline" })
    }
  }

  return tokens
}

/**
 * Theme colors for markdown rendering.
 */
export interface MarkdownTheme {
  text: string
  code: string
  codeBlockBg: string
  heading: string
  link: string
}

/**
 * Render a single inline token to JSX.
 * Used for simple inline content without block-level elements.
 */
export function renderInlineToken(token: Token, theme: MarkdownTheme): JSX.Element {
  switch (token.type) {
    case "text":
      return <span>{token.content}</span>
    case "bold":
      return <strong>{token.content}</strong>
    case "italic":
      return <em>{token.content}</em>
    case "code":
      // Inline code with background highlight
      return <span>{` ${token.content} `}</span>
    case "link":
      return <u>{token.text}</u>
    case "newline":
      return <br />
    case "heading":
      return <strong>{token.content}</strong>
    default:
      return <span>{(token as Token & { content?: string }).content || ""}</span>
  }
}

/**
 * Render inline tokens (for list items, etc.).
 */
export function renderInlineTokens(tokens: Token[], theme: MarkdownTheme): JSX.Element {
  return (
    <For each={tokens}>
      {(token) => renderInlineToken(token, theme)}
    </For>
  )
}

/**
 * Component that renders markdown content.
 * Handles both block-level and inline elements.
 */
export function MarkdownContent(props: { content: string; theme: MarkdownTheme }): JSX.Element {
  const tokens = () => parseMarkdown(props.content)

  return (
    <box flexDirection="column">
      <For each={tokens()}>
        {(token) => {
          switch (token.type) {
            case "codeblock":
              return (
                <box backgroundColor={props.theme.codeBlockBg} padding={1} marginTop={1} marginBottom={1}>
                  <text content={token.content} fg={props.theme.code} selectable />
                </box>
              )
            case "bullet":
              const bulletInline = parseInline(token.content)
              return (
                <text fg={props.theme.text} selectable>
                  {"• "}
                  {renderInlineTokens(bulletInline, props.theme)}
                </text>
              )
            case "numbered":
              const numberedInline = parseInline(token.content)
              return (
                <text fg={props.theme.text} selectable>
                  {`${token.number}. `}
                  {renderInlineTokens(numberedInline, props.theme)}
                </text>
              )
            case "heading":
              return (
                <text fg={props.theme.heading} selectable>
                  <strong>{token.content}</strong>
                </text>
              )
            case "newline":
              return <text content="" />
            default:
              return (
                <text fg={props.theme.text} selectable>
                  {renderInlineToken(token, props.theme)}
                </text>
              )
          }
        }}
      </For>
    </box>
  )
}
