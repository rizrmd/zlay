import { describe, it, expect, beforeEach, vi } from 'vitest'
import { webSocketService } from './websocket'

// Mock WebSocket
class MockWebSocket {
  static READY_STATES = {
    CONNECTING: 0,
    OPEN: 1,
    CLOSING: 2,
    CLOSED: 3
  }

  readyState = MockWebSocket.READY_STATES.OPEN
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor(url: string) {
    // Simulate successful connection
    setTimeout(() => {
      if (this.onopen) {
        this.onopen(new Event('open'))
      }
    }, 10)
  }

  send(data: string) {
    // Echo back the message for testing
    if (this.onmessage) {
      this.onmessage(new MessageEvent('message', { data }))
    }
  }

  close() {
    this.readyState = MockWebSocket.READY_STATES.CLOSED
    if (this.onclose) {
      this.onclose(new CloseEvent('close'))
    }
  }
}

// Setup global WebSocket mock
global.WebSocket = MockWebSocket as any

describe('WebSocket Service', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    // Reset service state
    webSocketService.disconnect()
  })

  it('should connect to WebSocket with project and token', async () => {
    const projectId = 'test-project'
    const token = 'test-token'

    await webSocketService.connect(projectId, token)

    expect(webSocketService.isConnected()).toBe(true)
  })

  it('should send and receive messages', async () => {
    const projectId = 'test-project'
    const token = 'test-token'
    let receivedMessage: any = null

    // Set up message handler
    webSocketService.onMessage('test_message', (data: any) => {
      receivedMessage = data
    })

    await webSocketService.connect(projectId, token)
    
    // Wait for connection to establish
    await new Promise(resolve => setTimeout(resolve, 20))

    webSocketService.sendMessage('test_message', { hello: 'world' })

    // Wait for message to be received
    await new Promise(resolve => setTimeout(resolve, 20))

    expect(receivedMessage).toEqual({ hello: 'world' })
  })

  it('should handle user messages correctly', async () => {
    const projectId = 'test-project'
    const token = 'test-token'

    await webSocketService.connect(projectId, token)
    
    // Wait for connection to establish
    await new Promise(resolve => setTimeout(resolve, 20))

    const conversationId = 'test-conversation'
    const content = 'Hello, assistant!'

    // This should not throw an error
    expect(() => {
      webSocketService.sendMessageToAssistant(conversationId, content)
    }).not.toThrow()
  })

  it('should handle conversation management', async () => {
    const projectId = 'test-project'
    const token = 'test-token'

    await webSocketService.connect(projectId, token)
    
    // Wait for connection to establish
    await new Promise(resolve => setTimeout(resolve, 20))

    // These should not throw errors
    expect(() => {
      webSocketService.createConversation('Test Conversation')
      webSocketService.getConversations()
      webSocketService.getConversation('test-conversation')
      webSocketService.deleteConversation('test-conversation')
    }).not.toThrow()
  })

  it('should handle project management', async () => {
    const projectId = 'test-project'
    const token = 'test-token'

    await webSocketService.connect(projectId, token)
    
    // Wait for connection to establish
    await new Promise(resolve => setTimeout(resolve, 20))

    // These should not throw errors
    expect(() => {
      webSocketService.joinProject('new-project')
      webSocketService.leaveProject()
    }).not.toThrow()
  })

  it('should disconnect properly', () => {
    expect(webSocketService.isConnected()).toBe(false)
    
    webSocketService.disconnect()
    
    expect(webSocketService.isConnected()).toBe(false)
  })
})