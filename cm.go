/******
This file is part of Vaelen/MUSH.

Copyright 2017, Andrew Young <andrew@vaelen.org>

    Vaelen/MUSH is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

    Vaelen/MUSH is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
along with Foobar.  If not, see <http://www.gnu.org/licenses/>.
******/

package mush

import (
	"log"
	"sync"
)

// ConnectionManager maintains open connections to the server
type ConnectionManager struct {
	connections      []*Connection
	connMutex        sync.RWMutex
	nextConnectionID IDType
	// Opened: Sending a ConnectionStateChange message to this channel adds the connection to the ConnectionManager.
	Opened           chan ConnectionStateChange
	// Closed: Sending a ConnectionStateChange message to this channel removes the connection from the ConnectionManager.
	Closed           chan ConnectionStateChange
	// Shutdown: Sending true to this channel shuts down the ConnectionManager.
	Shutdown         chan bool
}

// ConnectionStateChange is an event that is fired when a connection changes state.
type ConnectionStateChange struct {
	c   *Connection
	ack chan bool
}

// NewConnectionManager creates a new ConnectionManager instance.
func NewConnectionManager() *ConnectionManager {
	return  &ConnectionManager{
		nextConnectionID: 1,
		connections:      make([]*Connection, 0),
		Opened:           make(chan ConnectionStateChange),
		Closed:           make(chan ConnectionStateChange),
		Shutdown:         make(chan bool),
	}
}


// Connections returns a slice of the currently open connections.
func (m *ConnectionManager) Connections() []*Connection {
	m.connMutex.RLock()
	defer m.connMutex.RUnlock()
	c := make([]*Connection, 0, len(m.connections))
	c = append(c, m.connections...)
	return c
}

// findConnection iterates through the connections slice for a given connection id and returns the first matching connection.
func (m *ConnectionManager) findConnection(id IDType) int {
	i := -1
	for n, c := range m.connections {
		if c.ID == id {
			i = n
			break
		}
	}
	return i
}

// addConnection adds a connection to the connection manager's connections slice.
func (m *ConnectionManager) addConnection(c *Connection) {
	log.Println("Got Connection")
	m.connMutex.Lock()
	defer m.connMutex.Unlock()
	if m.findConnection(c.ID) > -1 {
		// Connection is already in the list
		return
	}
	i := m.nextConnectionID
	m.nextConnectionID++
	c.ID = i
	m.connections = append(m.connections, c)
	log.Printf("Open Connections: %d\n", len(m.connections))
}

// removeConnection removes a connection from the connection manager's connections slice.
func (m *ConnectionManager) removeConnection(c *Connection) {
	log.Println("Connection Closed")
	m.connMutex.Lock()
	defer m.connMutex.Unlock()
	// Find element to be removed
	i := m.findConnection(c.ID)
	if i < 0 {
		// Connection is not in the list
		return
	}
	// Delete the removed element (without possible memory leak)
	copy(m.connections[i:], m.connections[i+1:])
	m.connections[len(m.connections)-1] = nil
	m.connections = m.connections[:len(m.connections)-1]
	log.Printf("Open Connections: %d\n", len(m.connections))
}

// ConnectionManagerThread returns a goroutine for the Connection Manager.
// This goroutine must be running for the ConnectionManager to operate.
// Running multiple copies of this goroutine for the same ConnectionManager will produce unknown side effects.
func (m *ConnectionManager) ConnectionManagerThread() func() {
	return func() {
		log.Println("Connection Manager Started")
		defer log.Println("Connection Manager Stopped")
		for {
			select {
			case e := <-m.Opened:
				m.addConnection(e.c)
				e.ack <- true
			case e := <-m.Closed:
				m.removeConnection(e.c)
				e.ack <- true
			case <-m.Shutdown:
				for _, c := range m.Connections() {
					c.C.Close()
				}
				return
			}
		}
	}
}
