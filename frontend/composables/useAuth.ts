// useAuth — email/password + OAuth login; SIWE used only for wallet binding.
import { useAccount, useConnect, useSignMessage, useDisconnect } from '@wagmi/vue'
import { wagmiConfig } from '~/plugins/wagmi'
import type { User } from '~/types/api'

export function useAuth() {
  const api = useApi()
  const authStore = useAuthStore()
  const ui = useUiStore()
  const config = useRuntimeConfig()

  const { address, isConnected } = useAccount({ config: wagmiConfig })
  const { connectAsync, connectors } = useConnect({ config: wagmiConfig })
  const { signMessageAsync } = useSignMessage({ config: wagmiConfig })
  const { disconnect } = useDisconnect({ config: wagmiConfig })

  const loading = ref(false)

  // ── Email / password ──────────────────────────────────────────────────────

  async function login(identifier: string, password: string) {
    if (loading.value) return
    loading.value = true
    try {
      const { token, user } = await api.post<{ token: string; user: User }>(
        '/auth/login',
        { identifier, password }
      )
      authStore.setAuth(token, user)
      ui.toast('success', `Welcome back, ${user.username}!`)
      await navigateTo('/')
    } catch (err: any) {
      ui.toast('error', err?.message ?? 'Login failed')
      throw err
    } finally {
      loading.value = false
    }
  }

  async function register(email: string, password: string) {
    if (loading.value) return
    loading.value = true
    try {
      await api.post('/auth/register', { email, password })
      ui.toast('success', 'Registration successful — check your email to verify your account.')
    } catch (err: any) {
      ui.toast('error', err?.message ?? 'Registration failed')
      throw err
    } finally {
      loading.value = false
    }
  }

  // ── OAuth ─────────────────────────────────────────────────────────────────

  function loginWithOAuth(provider: 'google' | 'discord') {
    const apiBase = config.public.apiBase as string
    // Remove trailing /api/v1 if present to get the bare API origin.
    const apiOrigin = apiBase.replace(/\/api\/v1\/?$/, '')
    window.location.href = `${apiOrigin}/api/v1/auth/oauth/${provider}`
  }

  // ── Wallet binding (SIWE) ────────────────────────────────────────────────

  async function bindWallet() {
    if (loading.value) return
    if (!authStore.isLoggedIn) {
      ui.toast('error', 'You must be logged in to bind a wallet')
      return
    }
    loading.value = true
    try {
      // 1. Auto-trigger MetaMask connect if not yet connected. This opens the
      //    MetaMask popup to request account access from the user.
      //    Note: `connectors` from useConnect() is a plain array (already
      //    unwrapped internally — see @wagmi/vue useConnect.js), NOT a ref.
      if (!isConnected.value) {
        const list = connectors ?? []
        const metaMask = list.find(c => c.id === 'metaMaskSDK' || c.id === 'metaMask')
        if (!metaMask) {
          throw new Error('MetaMask not detected. Please install the MetaMask browser extension.')
        }
        ui.toast('info', 'Approve the MetaMask connection request…')
        await connectAsync({ connector: metaMask })
      }

      const wallet = address.value?.toLowerCase()
      if (!wallet) throw new Error('Wallet address not available after connecting')

      // 2. Ask the backend for a SIWE nonce + the exact message to sign.
      const { message } = await api.get<{ nonce: string; message: string; wallet: string }>(
        `/auth/wallet/nonce?wallet=${wallet}`
      )

      // 3. Open MetaMask again to sign the message (EIP-191 personal_sign).
      ui.toast('info', 'Sign the binding message in MetaMask…')
      const signature = await signMessageAsync({ message })

      // 4. Submit { wallet, signature, message } to complete binding.
      const data = await api.post<{ wallet_address: string }>('/auth/wallet/bind', {
        wallet,
        signature,
        message,
      })

      // 5. Refresh user profile so wallet_address appears immediately.
      const updatedUser = await api.get<User>('/users/me')
      authStore.setAuth(authStore.token!, updatedUser)

      ui.toast('success', `Wallet ${data.wallet_address.slice(0, 10)}… bound successfully`)
    } catch (err: any) {
      // Common wagmi/MetaMask user-reject codes; surface a friendlier message.
      const msg = err?.shortMessage || err?.message || 'Wallet binding failed'
      if (msg.includes('User rejected') || msg.includes('user rejected')) {
        ui.toast('info', 'Cancelled')
      } else {
        ui.toast('error', msg)
      }
    } finally {
      loading.value = false
    }
  }

  // ── Logout ────────────────────────────────────────────────────────────────

  function logout() {
    authStore.logout()
    if (isConnected.value) disconnect()
    ui.toast('info', 'Signed out')
    navigateTo('/')
  }

  return {
    loading,
    login,
    register,
    loginWithOAuth,
    bindWallet,
    logout,
    isLoggedIn: authStore.isLoggedIn,
    user: authStore.user,
  }
}
