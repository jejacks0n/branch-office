import colors from 'tailwindcss/colors'

// Theme-aware shades read from CSS variables defined in src/style.css, which
// swap under prefers-color-scheme so existing classes work in light and dark.
const v = (name) => `rgb(var(--${name}) / <alpha-value>)`
const themed = (name, shades) =>
  Object.fromEntries(shades.map((s) => [s, v(`${name}-${s}`)]))

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'Menlo', 'Monaco', 'Courier New', 'monospace'],
      },
      colors: {
        // Neutrals invert fully; accents only flip the light text shades and the
        // 950 tints. The 500/600 fills behind white text read fine on both.
        zinc: themed('zinc', [50, 100, 200, 300, 400, 500, 600, 700, 800, 900, 950]),
        emerald: { ...colors.emerald, ...themed('emerald', [200, 300, 400, 950]) },
        red: { ...colors.red, ...themed('red', [200, 300, 400, 950]) },
        amber: { ...colors.amber, ...themed('amber', [300, 400]) },
        purple: { ...colors.purple, ...themed('purple', [300, 400]) },
        teal: { ...colors.teal, ...themed('teal', [400, 950]) },
        sky: { ...colors.sky, ...themed('sky', [400]) },
      },
    },
  },
  plugins: [],
}
