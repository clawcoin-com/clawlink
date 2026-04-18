import type { Config } from 'tailwindcss'
import animate from 'tailwindcss-animate'

export default {
  darkMode: 'class',
  safelist: ['dark'],
  content: [
    './components/**/*.{js,vue,ts}',
    './layouts/**/*.vue',
    './pages/**/*.vue',
    './plugins/**/*.{js,ts}',
    './app.vue',
    './error.vue',
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['IBM Plex Mono', 'monospace'],
        mono: ['IBM Plex Mono', 'monospace'],
      },
      colors: {
        // ── Moltbook / ClawLink brand ──────────────────────
        moltbook: {
          dark:        '#1a1a1b',
          darker:      '#2d2d2e',
          red:         '#e01b24',
          'red-hover': '#ff3b3b',
          teal:        '#00d4aa',
          'teal-hover':'#00b894',
          blue:        '#4a9eff',
          gold:        '#ffd700',
          orange:      '#ff6b35',
          gray: {
            50:  '#fafafa',
            100: '#f5f5f5',
            200: '#e0e0e0',
            300: '#c0c0c0',
            400: '#888888',
            500: '#7c7c7c',
            600: '#666666',
            700: '#555555',
            800: '#444444',
            900: '#333333',
          },
        },
        // ── Vote colors ────────────────────────────────────
        upvote:   '#e01b24',
        downvote: '#4a9eff',
        // ── shadcn/Nuxt CSS-variable tokens ───────────────
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        card: {
          DEFAULT:    'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
        popover: {
          DEFAULT:    'hsl(var(--popover))',
          foreground: 'hsl(var(--popover-foreground))',
        },
        primary: {
          DEFAULT:    'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        secondary: {
          DEFAULT:    'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        muted: {
          DEFAULT:    'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        accent: {
          DEFAULT:    'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        destructive: {
          DEFAULT:    'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))',
        },
        border: 'hsl(var(--border))',
        input:  'hsl(var(--input))',
        ring:   'hsl(var(--ring))',
      },
      borderRadius: {
        xl: 'calc(var(--radius) + 6px)',
        lg: 'calc(var(--radius) + 2px)',
        md: 'var(--radius)',
        sm: 'calc(var(--radius) - 2px)',
      },
      keyframes: {
        'accordion-down': {
          from: { height: '0' },
          to:   { height: 'var(--radix-accordion-content-height)' },
        },
        'accordion-up': {
          from: { height: 'var(--radix-accordion-content-height)' },
          to:   { height: '0' },
        },
        'fade-in':  { from: { opacity: '0' }, to: { opacity: '1' } },
        'fade-out': { from: { opacity: '1' }, to: { opacity: '0' } },
        'slide-in-right':  { from: { transform: 'translateX(100%)' }, to: { transform: 'translateX(0)' } },
        'slide-out-right': { from: { transform: 'translateX(0)' }, to: { transform: 'translateX(100%)' } },
        'scale-in': {
          from: { transform: 'scale(0.95)', opacity: '0' },
          to:   { transform: 'scale(1)',    opacity: '1' },
        },
        shimmer: {
          '0%':   { transform: 'translateX(-100%)' },
          '100%': { transform: 'translateX(100%)' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0px)' },
          '50%':      { transform: 'translateY(-6px)' },
        },
        'pulse-glow': {
          '0%, 100%': { opacity: '1', transform: 'scale(1)' },
          '50%':      { opacity: '0.6', transform: 'scale(1.1)' },
        },
        ping: {
          '75%, 100%': { transform: 'scale(2)', opacity: '0' },
        },
      },
      animation: {
        'accordion-down':   'accordion-down 0.2s ease-out',
        'accordion-up':     'accordion-up 0.2s ease-out',
        'fade-in':          'fade-in 0.2s ease-out',
        'fade-out':         'fade-out 0.2s ease-out',
        'slide-in-right':   'slide-in-right 0.3s ease-out',
        'slide-out-right':  'slide-out-right 0.3s ease-out',
        'scale-in':         'scale-in 0.2s ease-out',
        shimmer:            'shimmer 2s infinite',
        float:              'float 3s ease-in-out infinite',
        'pulse-glow':       'pulse-glow 2s ease-in-out infinite',
        ping:               'ping 1s cubic-bezier(0, 0, 0.2, 1) infinite',
      },
    },
  },
  plugins: [animate],
} satisfies Config
