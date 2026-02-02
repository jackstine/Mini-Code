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
 * Scroll up by 10 viewports (PageUp).
 */
export function scrollPageUp() {
  const scrollbox = scrollboxRef()
  if (scrollbox) {
    scrollbox.scrollBy(-10, "viewport")
  }
}

/**
 * Scroll down by 10 viewports (PageDown).
 */
export function scrollPageDown() {
  const scrollbox = scrollboxRef()
  if (scrollbox) {
    scrollbox.scrollBy(10, "viewport")
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
    // Use a large numeric value instead of Infinity to avoid rendering issues
    scrollbox.scrollTo(999999)
  }
}

export { scrollboxRef }
