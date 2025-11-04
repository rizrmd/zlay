const API_BASE_URL = import.meta.env.DEV ? '' : window.location.origin

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

export interface Conversation {
  id: string
  title: string
  user_id: string
  project_id: string
  status: string // processing, completed, interrupted
  created_at: string
  updated_at: string
}

export interface ApiMessage {
  id: string
  conversation_id: string
  role: string
  content: string
  metadata?: Record<string, any>
  tool_calls?: ApiToolCall[]
  created_at: string
}

export interface ApiToolCall {
  id: string
  type: string
  function: {
    name: string
    arguments: any
  }
  status?: string
  result?: any
  error?: string
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
      const errorMessage =
        errorData.error || errorData.message || `HTTP error! status: ${response.status}`

      // Create specific error types for different scenarios
      if (response.status === 400 && errorMessage === 'Invalid client') {
        throw new Error('CLIENT_INVALID')
      } else if (response.status === 401) {
        throw new Error('AUTH_INVALID')
      } else {
        throw new Error(errorMessage)
      }
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
      throw new Error(
        errorData.error || errorData.message || `HTTP error! status: ${response.status}`,
      )
    }

    return response.json()
  }

  async logout(): Promise<AuthResponse> {
    return this.request<AuthResponse>('/api/auth/logout', {
      method: 'POST',
    })
  }

  async getProfile(): Promise<{ success: boolean; user?: UserProfile }> {
    try {
      return await this.request<{ success: boolean; user: UserProfile }>('/api/auth/profile')
    } catch (error) {
      if (error instanceof Error && error.message === 'Authentication required') {
        return { success: false }
      }
      throw error
    }
  }

  async getProjects(): Promise<Project[]> {
    return this.request<Project[]>('/api/projects')
  }

  async getConversations(): Promise<{ success: boolean; conversations?: Conversation[] }> {
    return this.request<{ success: boolean; conversations?: Conversation[] }>('/api/conversations')
  }

  async getConversationMessages(conversationId: string): Promise<{
    success: boolean
    conversation?: {
      conversation: Conversation
      messages: ApiMessage[]
    }
  }> {
    return this.request<{
      success: boolean
      conversation?: {
        conversation: Conversation
        messages: ApiMessage[]
      }
    }>(`/api/conversations/${conversationId}/messages`)
  }

  async createProject(
    name: string,
    description: string,
  ): Promise<{ success: boolean; message: string; project_id: string }> {
    return this.request<{ success: boolean; message: string; project_id: string }>(
      '/api/projects',
      {
        method: 'POST',
        body: JSON.stringify({ name, description }),
      },
    )
  }

  async checkHealth(): Promise<{ status: string }> {
    return this.request<{ status: string }>('/api/health')
  }
}

export const apiClient = new ApiClient()
