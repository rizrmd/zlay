package websocket

import (
	"encoding/json"
	"log"
	"sync"
)

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
	ID        string      `json:"id,omitempty"`
}

// Hub maintains the set of active connections and broadcasts messages to them
type Hub struct {
	// Registered connections
	connections map[*Connection]bool

	// Project-based rooms for isolation
	projects map[string]map[*Connection]bool

	// Inbound messages from the connections
	broadcast chan []byte

	// Register requests from the connections
	register chan *Connection

	// Unregister requests from connections
	unregister chan *Connection

	// Project join/leave requests
	projectJoin  chan *ProjectJoin
	projectLeave chan *ProjectLeave

	// Mutex for thread-safe operations
	mutex sync.RWMutex
}

// ProjectJoin represents a connection joining a project room
type ProjectJoin struct {
	Connection *Connection
	ProjectID  string
}

// ProjectLeave represents a connection leaving a project room
type ProjectLeave struct {
	Connection *Connection
	ProjectID  string
}

// NewHub creates a new hub instance
func NewHub() *Hub {
	return &Hub{
		connections:  make(map[*Connection]bool),
		projects:     make(map[string]map[*Connection]bool),
		broadcast:    make(chan []byte),
		register:     make(chan *Connection),
		unregister:   make(chan *Connection),
		projectJoin:  make(chan *ProjectJoin),
		projectLeave: make(chan *ProjectLeave),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mutex.Lock()
			h.connections[conn] = true
			h.mutex.Unlock()
			log.Printf("Connection registered: %s", conn.ID)

		case conn := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)

				// Remove from all project rooms
				for projectID, conns := range h.projects {
					if _, inRoom := conns[conn]; inRoom {
						delete(conns, conn)
						if len(conns) == 0 {
							delete(h.projects, projectID)
						}
					}
				}

				close(conn.send)
				h.mutex.Unlock()
				log.Printf("Connection unregistered: %s", conn.ID)
			}
			h.mutex.Unlock()

		case join := <-h.projectJoin:
			h.mutex.Lock()
			if h.projects[join.ProjectID] == nil {
				h.projects[join.ProjectID] = make(map[*Connection]bool)
			}
			h.projects[join.ProjectID][join.Connection] = true
			h.mutex.Unlock()
			log.Printf("Connection %s joined project %s", join.Connection.ID, join.ProjectID)

		case leave := <-h.projectLeave:
			h.mutex.Lock()
			if conns, exists := h.projects[leave.ProjectID]; exists {
				delete(conns, leave.Connection)
				if len(conns) == 0 {
					delete(h.projects, leave.ProjectID)
				}
			}
			h.mutex.Unlock()
			log.Printf("Connection %s left project %s", leave.Connection.ID, leave.ProjectID)

		case message := <-h.broadcast:
			h.mutex.RLock()
			for conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					// Connection send buffer is full, skip this connection
					close(conn.send)
					delete(h.connections, conn)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// BroadcastToProject sends a message to all connections in a project room
func (h *Hub) BroadcastToProject(projectID string, message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if conns, exists := h.projects[projectID]; exists {
		for conn := range conns {
			select {
			case conn.send <- data:
			default:
				// Connection send buffer is full
				close(conn.send)
				delete(conns, conn)
				delete(h.connections, conn)
			}
		}
	}
}

// SendToConnection sends a message to a specific connection
func (h *Hub) SendToConnection(conn *Connection, message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	select {
	case conn.send <- data:
	default:
		// Connection send buffer is full
		close(conn.send)
		h.mutex.Lock()
		delete(h.connections, conn)
		h.mutex.Unlock()
	}
}

// GetProjectConnectionCount returns the number of connections in a project room
func (h *Hub) GetProjectConnectionCount(projectID string) int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if conns, exists := h.projects[projectID]; exists {
		return len(conns)
	}
	return 0
}

// GetConnectionCount returns the total number of active connections
func (h *Hub) GetConnectionCount() int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	return len(h.connections)
}

// GetProjectConnections returns a copy of connections in a project room
func (h *Hub) GetProjectConnections(projectID string) []*Connection {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	var connections []*Connection
	if conns, exists := h.projects[projectID]; exists {
		for conn := range conns {
			connections = append(connections, conn)
		}
	}
	return connections
}
