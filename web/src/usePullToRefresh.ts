import { ref, computed, onMounted, onUnmounted, type Ref } from 'vue'

const DAMPING = 0.7 // finger travel -> content travel before resistance kicks in
const THRESHOLD = 56 // content offset past which releasing refreshes; also where it holds while loading
const RESISTANCE = 160 // asymptote of the rubber band: the content never travels further than this
const MIN_SPIN_MS = 500 // acknowledge the gesture even when the request returns instantly

/**
 * Pull-to-refresh for a scroll container. Touch only: a mouse has no pull gesture, and
 * dragging with one would fight text selection.
 *
 * Only engages when the container is scrolled to the very top, and hands control back to
 * native scrolling the moment the drag turns upward or mostly horizontal.
 */
export function usePullToRefresh(
  el: Ref<HTMLElement | null>,
  onRefresh: () => Promise<unknown>,
  enabled: () => boolean = () => true
) {
  const pull = ref(0) // indicator offset in px
  const dragging = ref(false) // finger down and pulling; callers should disable release easing
  const refreshing = ref(false)
  const progress = computed(() => Math.min(1, pull.value / THRESHOLD))

  let tracking = false
  let startX = 0
  let startY = 0

  function onStart(e: TouchEvent) {
    const node = el.value
    tracking = !!node && e.touches.length === 1 && node.scrollTop <= 0 && !refreshing.value && enabled()
    if (!tracking) return
    startX = e.touches[0].clientX
    startY = e.touches[0].clientY
  }

  function onMove(e: TouchEvent) {
    if (!tracking || !el.value) return
    const touch = e.touches[0]
    const dx = touch.clientX - startX
    const dy = touch.clientY - startY

    if (el.value.scrollTop > 0) {
      // Content scrolled under us mid-gesture: this is a normal scroll now.
      tracking = false
      dragging.value = false
      pull.value = 0
      return
    }
    if (!dragging.value && Math.abs(dx) > Math.abs(dy)) {
      tracking = false // horizontal swipe, not ours
      return
    }
    if (dy <= 0) {
      pull.value = 0 // dragged back up past the start: let native scrolling take it
      return
    }

    dragging.value = true
    // Rubber band: close to 1:1 at first, stiffening smoothly instead of hitting a hard stop.
    const raw = dy * DAMPING
    pull.value = raw / (1 + raw / RESISTANCE)
    // Suppress the native rubber-band while we own the gesture. Not always cancelable
    // (e.g. once a scroll is already underway), and cancelling then only logs a warning.
    if (e.cancelable) e.preventDefault()
  }

  async function onEnd() {
    const wasDragging = dragging.value
    tracking = false
    dragging.value = false
    if (!wasDragging) return

    if (pull.value < THRESHOLD) {
      pull.value = 0
      return
    }
    refreshing.value = true
    pull.value = THRESHOLD // settle to the hold position; content stays down while the request runs
    try {
      await Promise.all([onRefresh(), new Promise((r) => setTimeout(r, MIN_SPIN_MS))])
    } finally {
      refreshing.value = false
      pull.value = 0
    }
  }

  function onCancel() {
    tracking = false
    dragging.value = false
    if (!refreshing.value) pull.value = 0
  }

  // Captured at mount: Vue clears template refs before onUnmounted runs, so reading
  // el.value there would find null and leak these listeners.
  let bound: HTMLElement | null = null

  onMounted(() => {
    bound = el.value
    if (!bound) return
    bound.addEventListener('touchstart', onStart, { passive: true })
    // Must be non-passive or preventDefault above is ignored and iOS rubber-bands anyway.
    bound.addEventListener('touchmove', onMove, { passive: false })
    bound.addEventListener('touchend', onEnd)
    bound.addEventListener('touchcancel', onCancel)
  })

  onUnmounted(() => {
    if (!bound) return
    bound.removeEventListener('touchstart', onStart)
    bound.removeEventListener('touchmove', onMove)
    bound.removeEventListener('touchend', onEnd)
    bound.removeEventListener('touchcancel', onCancel)
    bound = null
  })

  return { pull, dragging, refreshing, progress }
}
