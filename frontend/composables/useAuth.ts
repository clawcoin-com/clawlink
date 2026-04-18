// useAuth — email/password + OAuth login; SIWE used only for wallet binding.
import { useAccount, useSignMessage, useDisconnect } from '@wagmi/vue'
import { wagmiConfig } from '~/plugins/wagmi.client'
import type { User } from '~/types/api'

export function useAuth() {
  const api = useApi()
  const authStore = useAuthStore()
  const ui = useUiStore()
  const config = useRuntimeConfig()

  const { address, isConnected } = useAccount({ config: wagmiConfig })
  const { signMessageAsync } = useSignMessage({ config: wagmiConfig })
  const { disconnect } = useDisconnect({ config: wagmiConfig })

  const loading = ref(false)

  // ── Email / password ──────────────────────────────────────────────────────

  async function loginWithEmail(email: string, password: string) {
    if (loading.value) return
    loading.value = true
    try {
      const { token, user } = await api.post<{ token: string; user: User }>(
        '/auth/login',
        { email, password }
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
      // Ensure MetaMask is connected.
      if (!isConnected.value) {
        ui.toast('info', 'Open MetaMask and connect your wallet first')
        return
      }

      const wallet = address.value?.toLowerCase()
      if (!wallet) throw new Error('Wallet address not available')

      // Get SIWE nonce from the binding endpoint.
      const { nonce, message } = await api.get<{ nonce: string; message: string }>(
        `/auth/wallet/nonce?wallet=${wallet}`
      )

      // Sign with MetaMask.
      const signature = await signMessageAsync({ message })

      // Submit binding.
      const data = await api.post<{ wallet_address: string }>('/auth/wallet/bind', {
        wallet,
        signature,
        message,
      })

      // Refresh user profile so wallet_address appears immediately.
      const updatedUser = await api.get<User>('/users/me')
      authStore.setAuth(authStore.token!, updatedUser)

      ui.toast('success', `Wallet ${data.wallet_address.slice(0, 8)}… bound successfully`)
    } catch (err: any) {
      ui.toast('error', err?.message ?? 'Wallet binding failed')
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
    loginWithEmail,
    register,
    loginWithOAuth,
    bindWallet,
    logout,
    isLoggedIn: authStore.isLoggedIn,
    user: authStore.user,
  }
}
