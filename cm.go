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

type ConnectionManager struct {
	connections []*Connection
	connMutex sync.RWMutex
	nextConnectionId IdType
	Opened chan ConnectionStateChange
	Closed chan ConnectionStateChange
	Shutdown chan bool
}

type ConnectionStateChange struct {
	c *Connection
	ack chan bool
}

func (m *ConnectionManager) Connections() []*Connection {
	m.connMutex.RLock()
	defer m.connMutex.RUnlock()
	c := make([]*Connection, 0, len(m.connections))
	c = append(c, m.connections...)
	return c
}

// This method is not threadsafe
func (m *ConnectionManager) findConnection(id IdType) int {
	i := -1
	for n, c := range m.connections {
		if c.Id == id {
			i = n
			break
		}
	}
	return i
}

func (m *ConnectionManager) AddConnection(c *Connection) {
	log.Println("Got Connection")
	m.connMutex.Lock()
	defer m.connMutex.Unlock()
	if m.findConnection(c.Id) > -1 {
		// Connection is already in the list
		return
	}
	i := m.nextConnectionId
	m.nextConnectionId++
	c.Id = i
	m.connections = append(m.connections, c)
	log.Printf("Open Connections: %d\n", len(m.connections))	
}

func (m *ConnectionManager) RemoveConnection(c *Connection) {
	log.Println("Connection Closed")
	m.connMutex.Lock()
	defer m.connMutex.Unlock()
	// Find element to be removed
	i := m.findConnection(c.Id)
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

func (m *ConnectionManager) ConnectionManagerThread() func() {
	return func() {
		log.Println("Connection Manager Started")
		defer log.Println("Connection Manager Stopped")
		for {
			select {
			case e := <-m.Opened:
				m.AddConnection(e.c)
				e.ack <- true
			case e := <-m.Closed:
				m.RemoveConnection(e.c)
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
