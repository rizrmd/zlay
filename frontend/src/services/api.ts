const API_BASE_URL = ''

export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  email: string
  password: string
  first_name?: string
  last_name?: string
}

export interface AuthResponse {
  success: boolean
  message: string
  user?: UserProfile
}

export interface UserProfile {
  id: string
  email: string
  first_name?: string
  last_name?: string
  created_at: string
}

class ApiClient {
  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${API_BASE_URL}${endpoint}`

    const config: RequestInit = {
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
      credentials: 'include',
      ...options,
    }

    const response = await fetch(url, config)

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    return response.json()
  }

  async login(credentials: LoginRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify(credentials),
    })
  }

  async register(userData: RegisterRequest): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify(userData),
    })
  }

  async logout(): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/logout', {
      method: 'POST',
    })
  }

  async getProfile(): Promise<{ success: boolean; user: UserProfile }> {
    return this.request<{ success: boolean; user: UserProfile }>('/api/auth/profile')
  }

  async checkHealth(): Promise<{ status: string }> {
    return this.request<{ status: string }>('/api/health')
  }
}

export const apiClient = new ApiClient()
