// useAuth — SIWE complete login flow using wagmi.
// Flow: connect wallet → get nonce → sign message → verify → store JWT
import { useConnect, useAccount, useSignMessage, useDisconnect } from '@wagmi/vue'
import { wagmiConfig } from '~/plugins/wagmi.client'

export function useAuth() {
  const api = useApi()
  const authStore = useAuthStore()
  const ui = useUiStore()

  const { connect, connectors } = useConnect({ config: wagmiConfig })
  const { address, isConnected } = useAccount({ config: wagmiConfig })
  const { signMessageAsync } = useSignMessage({ config: wagmiConfig })
  const { disconnect } = useDisconnect({ config: wagmiConfig })

  const loading = ref(false)

  async function login() {
    if (loading.value) return
    loading.value = true
    try {
      // 1. Connect wallet if not connected
      if (!isConnected.value) {
        const metamaskConnector = connectors.find(c => c.id === 'io.metamask' || c.id === 'metaMask')
        if (!metamaskConnector) {
          ui.toast('error', 'MetaMask not found. Please install it.')
          return
        }
        await connect({ connector: metamaskConnector })
        // Wait a tick for address to resolve
        await new Promise(r => setTimeout(r, 300))
      }

      const wallet = address.value?.toLowerCase()
      if (!wallet) throw new Error('Wallet address not available')

      // 2. Get nonce + SIWE message from API
      const { nonce, message } = await api.get<{ nonce: string; message: string; wallet: string }>(
        `/auth/nonce?wallet=${wallet}`
      )

      // 3. Sign the SIWE message
      const signature = await signMessageAsync({ message })

      // 4. Verify signature → get JWT
      const { token, user } = await api.post<{ token: string; user: any }>('/auth/verify', {
        wallet,
        signature,
        message,
      })

      authStore.setAuth(token, user)
      ui.toast('success', `Signed in as ${user.username}`)
    } catch (err: any) {
      ui.toast('error', err?.message ?? 'Login failed')
    } finally {
      loading.value = false
    }
  }

  function logout() {
    authStore.logout()
    disconnect()
    ui.toast('info', 'Signed out')
  }

  return { login, logout, loading, isLoggedIn: authStore.isLoggedIn, user: authStore.user }
}
