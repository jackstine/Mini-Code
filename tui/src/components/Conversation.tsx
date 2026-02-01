import type { Component } from "solid-js"
import { For, Switch, Match, onMount } from "solid-js"
import { parts } from "../stores/conversation"
import { setScrollbox } from "../stores/scroll"
import { UserPart } from "./parts/UserPart"
import { TextPart } from "./parts/TextPart"
import { ToolPart } from "./parts/ToolPart"
import { ReasoningPart } from "./parts/ReasoningPart"
import { theme } from "../theme"

/**
 * Conversation displays all parts in a scrollable container.
 * Uses stickyScroll to auto-scroll on new content.
 * Auto-scroll pauses when user scrolls up, resumes when at bottom.
 * Exposes scrollbox ref for programmatic scrolling via scroll store.
 */
export const Conversation: Component = () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let scrollboxRef: any

  onMount(() => {
    // Register scrollbox ref with the scroll store for global keyboard control
    if (scrollboxRef) {
      setScrollbox(scrollboxRef)
    }
  })

  return (
    <scrollbox
      ref={scrollboxRef}
      width="100%"
      flexGrow={1}
      stickyScroll={true}
      borderStyle="single"
      borderColor={theme.colors.border}
      padding={1}
    >
    <box flexDirection="column">
      <For each={parts}>
        {(part) => (
          <Switch>
            <Match when={part.type === "user" && part}>
              {(p) => <UserPart content={p().content} />}
            </Match>
            <Match when={part.type === "text" && part}>
              {(p) => <TextPart content={p().content} />}
            </Match>
            <Match when={part.type === "tool" && part}>
              {(p) => (
                <ToolPart
                  name={p().name}
                  input={p().input}
                  result={p().result}
                  isError={p().isError}
                />
              )}
            </Match>
            <Match when={part.type === "reasoning" && part}>
              {(p) => <ReasoningPart content={p().content} />}
            </Match>
          </Switch>
        )}
      </For>
    </box>
    </scrollbox>
  )
}
