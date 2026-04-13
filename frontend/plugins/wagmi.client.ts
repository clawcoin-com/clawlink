// wagmi config — client-side only (`.client.ts` suffix prevents SSR execution).
// WagmiPlugin internally uses @tanstack/vue-query, so VueQueryPlugin must be
// registered first — otherwise any useConnect/useAccount call throws
// "No 'queryClient' found in Vue context".
import { WagmiPlugin, createConfig, http } from '@wagmi/vue'
import { VueQueryPlugin } from '@tanstack/vue-query'
import { metaMask, walletConnect } from '@wagmi/connectors'
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
    // walletConnect({ projectId: 'YOUR_WC_PROJECT_ID' }),  // enable when WalletConnect project ID is configured
  ],
})

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp.use(VueQueryPlugin)
  nuxtApp.vueApp.use(WagmiPlugin, { config: wagmiConfig })
})
