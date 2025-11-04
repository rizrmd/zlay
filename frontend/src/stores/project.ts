import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import webSocketService from '@/services/websocket'

export const useProjectStore = defineStore('project', () => {
  // State
  const currentProjectId = ref<string | null>(null)
  const isConnected = ref(false)
  const connectionStatus = ref<string>('disconnected')
  const projects = ref<Map<string, any>>(new Map())
  
  // Loading states
  const isLoadingProjects = ref(false)
  const isConnecting = ref(false)
  
  // Computed
  const currentProject = computed(() => {
    if (!currentProjectId.value) return null
    return projects.value.get(currentProjectId.value) || null
  })
  
  const hasProject = computed(() => currentProjectId.value !== null)
  const canConnect = computed(() => currentProjectId.value !== null && !isConnecting.value)
  
  // Actions
  const setCurrentProject = (projectId: string) => {
    currentProjectId.value = projectId
    console.log('🏗️ PROJECT STORE: Current project set to:', projectId)
  }
  
  const clearCurrentProject = () => {
    currentProjectId.value = null
    isConnected.value = false
    connectionStatus.value = 'disconnected'
  }
  
  const initWebSocket = async (projectId: string) => {
    if (!projectId) {
      console.error('🏗️ PROJECT STORE: Project ID required for WebSocket connection')
      return
    }
    
    try {
      console.log('🏗️ PROJECT STORE: Initializing WebSocket for project:', projectId)
      isConnecting.value = true
      setCurrentProject(projectId)
      
      await webSocketService.connect(projectId)
      isConnected.value = true
      connectionStatus.value = 'connected'
      
      console.log('🏗️ PROJECT STORE: WebSocket connection successful')
    } catch (error) {
      console.error('🏗️ PROJECT STORE: Failed to initialize WebSocket:', error)
      isConnected.value = false
      connectionStatus.value = 'error'
    } finally {
      isConnecting.value = false
    }
  }
  
  const disconnectWebSocket = () => {
    webSocketService.disconnect()
    isConnected.value = false
    connectionStatus.value = 'disconnected'
    console.log('🏗️ PROJECT STORE: WebSocket disconnected')
  }
  
  const joinProject = (projectId: string) => {
    console.log('🏗️ PROJECT STORE: Joining project:', projectId)
    webSocketService.joinProject(projectId)
  }
  
  const leaveProject = () => {
    console.log('🏗️ PROJECT STORE: Leaving project')
    webSocketService.leaveProject()
  }
  
  const loadProjects = async () => {
    try {
      console.log('🏗️ PROJECT STORE: Loading projects')
      isLoadingProjects.value = true
      
      // This would typically call an API endpoint
      // For now, projects are managed through URL routing
      console.log('🏗️ PROJECT STORE: Projects loaded from URL context')
      
    } catch (error) {
      console.error('🏗️ PROJECT STORE: Error loading projects:', error)
    } finally {
      isLoadingProjects.value = false
    }
  }
  
  // WebSocket event handlers
  const setupWebSocketHandlers = () => {
    // Project joined confirmation
    webSocketService.onMessage('project_joined', (data: any) => {
      console.log('🏗️ PROJECT STORE: Project joined successfully:', data)
      if (data.project_id) {
        // Add project to store if not already present
        if (!projects.value.has(data.project_id)) {
          projects.value.set(data.project_id, {
            id: data.project_id,
            name: data.project_name || `Project ${data.project_id}`,
            joined_at: new Date().toISOString(),
          })
        }
      }
    })
    
    // Connection established
    webSocketService.onMessage('connection_established', (data: any) => {
      console.log('🏗️ PROJECT STORE: Connection established:', data)
      if (data.project_id) {
        joinProject(data.project_id)
      }
    })
    
    // Connection status updates
    webSocketService.onMessage('connection_status', (data: any) => {
      console.log('🏗️ PROJECT STORE: Connection status update:', data)
      connectionStatus.value = data.status || 'unknown'
    })
    
    // Error handling
    webSocketService.onMessage('project_error', (data: any) => {
      console.error('🏗️ PROJECT STORE: Project error:', data)
      isConnected.value = false
      connectionStatus.value = 'error'
    })
  }
  
  // Health check
  const pingProject = () => {
    console.log('🏗️ PROJECT STORE: Sending ping to project')
    webSocketService.sendPing()
  }
  
  // Cleanup
  const cleanup = () => {
    console.log('🏗️ PROJECT STORE: Cleaning up project store')
    disconnectWebSocket()
    clearCurrentProject()
  }
  
  return {
    // State
    currentProjectId,
    currentProject,
    projects,
    isConnected,
    connectionStatus,
    isConnecting,
    isLoadingProjects,
    
    // Computed
    hasProject,
    canConnect,
    
    // Actions
    setCurrentProject,
    clearCurrentProject,
    initWebSocket,
    disconnectWebSocket,
    joinProject,
    leaveProject,
    loadProjects,
    setupWebSocketHandlers,
    pingProject,
    cleanup,
  }
})