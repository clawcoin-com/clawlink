// wagmi config — runs on BOTH server and client so SSR pages that call
// useAuth()/useAccount() don't crash with "No 'queryClient' found".
//
// WagmiPlugin internally uses @tanstack/vue-query, so VueQueryPlugin must be
// registered first. Wagmi v2 is SSR-safe when `ssr: true` is set on the config.
import { WagmiPlugin, createConfig, http } from '@wagmi/vue'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { metaMask } from '@wagmi/connectors'
import { defineChain } from 'viem'

const clawCoinTestnet = defineChain({
  id: 11111110,
  name: 'ClawCoin Testnet',
  nativeCurrency: { name: 'ClawCoin', symbol: 'CC', decimals: 18 },
  rpcUrls: {
    default: { http: ['https://evm-testnet.clawcoin.com'] },
  },
  blockExplorers: {
    default: { name: 'ClawScan', url: 'https://scan.clawcoin.com' },
  },
  testnet: true,
})

export const wagmiConfig = createConfig({
  chains: [clawCoinTestnet],
  transports: { [clawCoinTestnet.id]: http() },
  connectors: [
    metaMask(),
    // walletConnect({ projectId: 'YOUR_WC_PROJECT_ID' }),  // enable when WC project ID is configured
  ],
  // Required for SSR so wagmi doesn't try to touch localStorage/window
  // during server-side rendering.
  ssr: true,
})

export default defineNuxtPlugin((nuxtApp) => {
  // VueQueryPlugin MUST come first — WagmiPlugin depends on the QueryClient.
  nuxtApp.vueApp.use(VueQueryPlugin)
  nuxtApp.vueApp.use(WagmiPlugin, { config: wagmiConfig })
})
