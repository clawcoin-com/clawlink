// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-01-01',
  ssr: true,

  css: ['~/assets/css/globals.css'],

  modules: [
    '@nuxtjs/tailwindcss',
    '@nuxtjs/color-mode',
    // '@vite-pwa/nuxt',  // enable on Node 20/22; Node 24 has object-hash/crypto incompatibility
    '@pinia/nuxt',
    'shadcn-nuxt',
  ],

  // Tell @nuxtjs/tailwindcss to use globals.css as its entry (avoids duplicate injection)
  tailwindcss: {
    cssPath: '~/assets/css/globals.css',
  },

  shadcn: {
    prefix: '',
    componentDir: './components/ui',
  },

  colorMode: {
    classSuffix: '',
    preference: 'dark',
    fallback: 'dark',
  },

  runtimeConfig: {
    // Server-only (SSR → Docker internal network).  Never sent to browser.
    apiBase: process.env.NUXT_API_BASE ?? 'http://localhost:8080/api/v1',
    public: {
      // Client-side (browser → nginx proxy).  Also used as SSR fallback when
      // NUXT_API_BASE is not set (e.g. local `npm run dev`).
      apiBase: process.env.NUXT_PUBLIC_API_URL ?? 'http://localhost:8080/api/v1',
      clawcoinRpc: process.env.NUXT_PUBLIC_CLAWCOIN_RPC ?? 'https://evm-testnet.clawcoin.com',
      clawcoinChainId: Number(process.env.NUXT_PUBLIC_CLAWCOIN_CHAIN_ID ?? '11111110'),
    },
  },

  // pwa: { ... }  — re-enable after adding @vite-pwa/nuxt to modules (requires Node 20/22)

  // Google Fonts — IBM Plex Mono
  app: {
    head: {
      titleTemplate: '%s',
      meta: [
        { name: 'theme-color', content: '#0f0f0f' },
        { name: 'apple-mobile-web-app-capable', content: 'yes' },
        { name: 'apple-mobile-web-app-status-bar-style', content: 'black-translucent' },
        { property: 'og:image', content: '/icon-512x512.png' },
        { name: 'twitter:image', content: '/icon-512x512.png' },
      ],
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
        { rel: 'icon', type: 'image/png', sizes: '32x32', href: '/favicon.png' },
        { rel: 'icon', type: 'image/png', sizes: '16x16', href: '/icon-16x16.png' },
        { rel: 'apple-touch-icon', sizes: '180x180', href: '/icon-180x180.png' },
        { rel: 'manifest', href: '/site.webmanifest' },
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=IBM+Plex+Mono:wght@400;500;600;700&display=swap',
        },
        {
          rel: 'stylesheet',
          href: 'https://cdn.jsdelivr.net/npm/remixicon@4.5.0/fonts/remixicon.css',
        },
      ],
    },
  },

  // Vite config — required for wagmi/viem in SSR.
  // All wagmi/viem packages must be in optimizeDeps.include so Vite pre-bundles
  // them before the first build pass, preventing the "needs 3 builds" syndrome
  // caused by CJS→ESM conversion being discovered mid-build.
  vite: {
    optimizeDeps: {
      include: [
        '@wagmi/vue',
        '@wagmi/core',
        '@wagmi/connectors',
        'viem',
        '@tanstack/vue-query',
      ],
    },
  },
})
