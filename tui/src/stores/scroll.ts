import { createSignal } from "solid-js"

/**
 * Scroll store manages the scrollbox reference and provides
 * programmatic scroll control functions.
 *
 * The scrollbox reference is set by the Conversation component
 * and used by the App component for keyboard navigation.
 */

// Store the scrollbox instance reference
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const [scrollboxRef, setScrollboxRef] = createSignal<any>(null)

/**
 * Set the scrollbox reference (called by Conversation component).
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function setScrollbox(ref: any) {
  setScrollboxRef(ref)
}

/**
 * Scroll up by one viewport (PageUp).
 */
export function scrollPageUp() {
  const scrollbox = scrollboxRef()
  if (scrollbox) {
    scrollbox.scrollBy(-1, "viewport")
  }
}

/**
 * Scroll down by one viewport (PageDown).
 */
export function scrollPageDown() {
  const scrollbox = scrollboxRef()
  if (scrollbox) {
    scrollbox.scrollBy(1, "viewport")
  }
}

/**
 * Scroll to the top of the content (Home).
 */
export function scrollToTop() {
  const scrollbox = scrollboxRef()
  if (scrollbox) {
    scrollbox.scrollTo(0)
  }
}

/**
 * Scroll to the bottom of the content (End).
 */
export function scrollToBottom() {
  const scrollbox = scrollboxRef()
  if (scrollbox) {
    // Use a large value to scroll to end
    scrollbox.scrollTo({ y: Infinity })
  }
}

export { scrollboxRef }
