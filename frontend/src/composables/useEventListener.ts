import { getCurrentInstance, onMounted, onUnmounted } from 'vue'

/**
 * Composable for safely registering and cleaning up DOM/MediaQuery event listeners.
 */
export function useEventListener<E extends Event = Event>(
  target: EventTarget | null | undefined,
  event: string,
  callback: (event: E) => void,
) {
  const listener = callback as EventListener

  if (getCurrentInstance()) {
    onMounted(() => {
      target?.addEventListener?.(event, listener)
    })

    onUnmounted(() => {
      target?.removeEventListener?.(event, listener)
    })
  }
}
