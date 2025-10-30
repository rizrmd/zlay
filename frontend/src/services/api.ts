const API_BASE_URL = import.meta.env.DEV ? 'http://localhost:8080' : window.location.origin

export interface LoginRequest {
  username: string
  password: string
}

export interface RegisterRequest {
  username: string
  password: string
}

export interface AuthResponse {
  success: boolean
  message: string
  user?: UserProfile
}

export interface UserProfile {
  id: string
  username: string
  created_at: string
}

export interface Project {
  id: string
  user_id: string
  name: string
  description: string
  is_active: boolean
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
      if (response.status === 401) {
        // Unauthorized - redirect to login
        throw new Error('Authentication required')
      }
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    return response.json()
  }

  async login(credentials: LoginRequest): Promise<AuthResponse> {
    const url = `${API_BASE_URL}/api/auth/login`
    
    const config: RequestInit = {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify(credentials),
    }

    const response = await fetch(url, config)

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      throw new Error(errorData.error || errorData.message || `HTTP error! status: ${response.status}`)
    }

    return response.json()
  }

  async register(userData: RegisterRequest): Promise<AuthResponse> {
    const url = `${API_BASE_URL}/api/auth/register`
    
    const config: RequestInit = {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      credentials: 'include',
      body: JSON.stringify(userData),
    }

    const response = await fetch(url, config)

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}))
      throw new Error(errorData.error || errorData.message || `HTTP error! status: ${response.status}`)
    }

    return response.json()
  }

  async logout(): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/logout', {
      method: 'POST',
    })
  }

  async getProfile(): Promise<{ success: boolean; user: UserProfile }> {
    return this.request<{ success: boolean; user: UserProfile }>('/api/auth/profile')
  }

  async getProjects(): Promise<Project[]> {
    return this.request<Project[]>('/api/projects')
  }

  async createProject(name: string, description: string): Promise<{ success: boolean; message: string; project_id: string }> {
    return this.request<{ success: boolean; message: string; project_id: string }>('/api/projects', {
      method: 'POST',
      body: JSON.stringify({ name, description }),
    })
  }

  async checkHealth(): Promise<{ status: string }> {
    return this.request<{ status: string }>('/api/health')
  }
}

export const apiClient = new ApiClient()
