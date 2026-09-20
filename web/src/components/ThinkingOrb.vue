<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { resolvePreset, MODE_DRAWS } from 'thinking-orbs/engine'
import type { OrbState, OrbSize } from 'thinking-orbs/engine'

const props = withDefaults(
  defineProps<{
    state?: OrbState
    size?: OrbSize | number
    speed?: number
    paused?: boolean
    dark?: boolean
  }>(),
  {
    state: 'working',
    size: 20,
    speed: 1,
    paused: false,
    dark: true,
  }
)

const canvasRef = ref<HTMLCanvasElement | null>(null)
let animId: number | null = null
let isRunning = false

function getPresetSize(size: number): 20 | 64 {
  return size >= 42 ? 64 : 20
}

function render(ctx: CanvasRenderingContext2D, dpr: number) {
  const s = props.size as number
  const presetSize = getPresetSize(s)
  const { mode, speed: presetSpeed, opts } = resolvePreset(props.state as any, presetSize)
  const draw = MODE_DRAWS[mode]
  const clock = (performance.now() / 1000) * presetSpeed * props.speed
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, s, s)
  if (draw) {
    draw(ctx, s, clock, props.dark, opts)
  }
}

function loop() {
  if (!isRunning || !canvasRef.value) return
  const ctx = canvasRef.value.getContext('2d')
  if (ctx) {
    const dpr = Math.min(2, typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1)
    render(ctx, dpr)
  }
  animId = requestAnimationFrame(loop)
}

function start() {
  if (isRunning || props.paused) return
  isRunning = true
  animId = requestAnimationFrame(loop)
}

function stop() {
  isRunning = false
  if (animId !== null) {
    cancelAnimationFrame(animId)
    animId = null
  }
}

function setupCanvas() {
  if (!canvasRef.value) return
  const dpr = Math.min(2, typeof window !== 'undefined' ? window.devicePixelRatio || 1 : 1)
  const s = props.size as number
  canvasRef.value.width = Math.round(s * dpr)
  canvasRef.value.height = Math.round(s * dpr)
  const ctx = canvasRef.value.getContext('2d')
  if (ctx) {
    render(ctx, dpr)
  }
}

const handleVisibility = () => {
  if (document.visibilityState === 'visible') {
    start()
  } else {
    stop()
  }
}

watch(
  () => [props.state, props.size, props.speed, props.dark],
  () => {
    setupCanvas()
  }
)

watch(
  () => props.paused,
  (paused) => {
    if (paused) stop()
    else start()
  }
)

onMounted(() => {
  setupCanvas()
  start()
  document.addEventListener('visibilitychange', handleVisibility)
})

onUnmounted(() => {
  stop()
  document.removeEventListener('visibilitychange', handleVisibility)
})
</script>

<template>
  <canvas
    ref="canvasRef"
    role="img"
    :aria-label="state"
    :style="{ width: `${size}px`, height: `${size}px`, display: 'inline-block', verticalAlign: 'middle' }"
  />
</template>
