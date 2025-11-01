import { ref, computed, readonly } from 'vue'
import { apiClient, type UserProfile } from '@/services/api'

const user = ref<UserProfile | null>(null)
const isLoading = ref(false)
let checkAuthPromise: Promise<boolean> | null = null

export const useAuth = () => {
  const isAuthenticated = computed(() => !!user.value)

  const login = async (username: string, password: string) => {
    isLoading.value = true
    try {
      const response = await apiClient.login({ username, password })
      console.log('Login response:', response)
      if (response.success && response.user) {
        user.value = response.user
        return { success: true }
      }
      return { success: false, message: response.message || 'Login failed' }
    } catch (error) {
      console.error('Login error:', error)
      let errorMessage = 'Network error occurred'

      if (error instanceof Error) {
        switch (error.message) {
          case 'CLIENT_INVALID':
            errorMessage = 'This application is not properly configured for your domain. Please contact your administrator.'
            break
          case 'AUTH_INVALID':
            errorMessage = 'Invalid username or password'
            break
          default:
            errorMessage = error.message
        }
      }

      return { success: false, message: errorMessage }
    } finally {
      isLoading.value = false
    }
  }

  const register = async (username: string, password: string) => {
    isLoading.value = true
    try {
      const response = await apiClient.register({ username, password })
      if (response.success) {
        return { success: true }
      }
      return { success: false, message: response.message }
    } catch (error) {
      console.error('Registration error:', error)
      const errorMessage = error instanceof Error ? error.message : 'Network error occurred'
      return { success: false, message: errorMessage }
    } finally {
      isLoading.value = false
    }
  }

  const logout = async () => {
    try {
      await apiClient.logout()
    } catch (error) {
      console.error('Logout error:', error)
    } finally {
      user.value = null
    }
  }

  const checkAuth = async () => {
    // Return existing promise if checkAuth is already in progress
    if (checkAuthPromise) {
      return checkAuthPromise
    }
    
    checkAuthPromise = (async () => {
      try {
        // Try to validate with server (will check cookies)
        const response = await apiClient.getProfile()
        console.log('Check auth response:', response)
        if (response.success && response.user) {
          user.value = response.user
          return true
        }
        return false
      } catch (error) {
        // Only log unexpected errors, not authentication failures
        if (!(error instanceof Error) || error.message !== 'Authentication required') {
          console.error('Check auth error:', error)
        }
        user.value = null
        return false
      } finally {
        checkAuthPromise = null
      }
    })()
    
    return checkAuthPromise
  }

  return {
    user: readonly(user),
    isLoading: readonly(isLoading),
    isAuthenticated,
    login,
    register,
    logout,
    checkAuth,
  }
}
