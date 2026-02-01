import type { Component } from "solid-js"
import { MarkdownContent, type MarkdownTheme } from "../../lib/markdown"
import { theme } from "../../theme"

interface Props {
  content: string
}

/**
 * TextPart displays agent text output with markdown rendering.
 * Supports bold, italic, inline code, code blocks, lists, headings, and links.
 */
export const TextPart: Component<Props> = (props) => {
  const markdownTheme: MarkdownTheme = {
    text: theme.colors.text,
    code: theme.colors.text,
    codeBlockBg: theme.colors.codeBlockBg,
    heading: theme.colors.text,
    link: theme.colors.userPrompt, // Use cyan for links
  }

  return (
    <box flexDirection="column" marginBottom={1}>
      <MarkdownContent content={props.content} theme={markdownTheme} />
    </box>
  )
}
