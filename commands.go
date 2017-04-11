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
	"fmt"
	"github.com/abiosoft/ishell"
	"strings"
	"time"
)

func addCommands(c *Connection) {
	shell := c.Shell
	player := c.Player

	shell.AddCmd(&ishell.Cmd{
		Name: "exit",
		Help: "Log off",
		Func: func(e *ishell.Context) {
			e.Printf("Goodbye, %s\n", player.Name)
			e.Stop()
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "say",
		Help: "Say something to the everybody else. Usage: say [player] <message>",
		Func: func(e *ishell.Context) {
			if len(e.Args) > 0 {
				var target string
				var phrase string
				if len(e.Args) > 1 {
					target = e.Args[0]
					phrase = e.Args[1]
				} else {
					target = ""
					phrase = e.Args[0]
				}
				c.Log("Executing Say: %s - %s", target, phrase)
				c.Say(target, phrase)
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "whisper",
		Help: "Whisper something to the somebody else. Usage: whisper <player> <message>",
		Func: func(e *ishell.Context) {
			if len(e.Args) > 1 {
				c.updateIdleTime()
				target := e.Args[0]
				phrase := e.Args[1]
				c.Log("Executing Whisper: %s - %s", target, phrase)
				c.Whisper(target, phrase)
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "emote",
		Help: "Do something. Usage: emote <action>",
		Func: func(e *ishell.Context) {
			if len(e.Args) > 0 {
				c.updateIdleTime()
				action := e.Args[0]
				c.Log("Executing Emote: %s", action)
				c.Emote(action)
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "look",
		Help: "Look around",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			c.Look()
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "who",
		Help: "See who's online",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			c.Who()
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "save",
		Help: "Save world state (admin)",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if c.IsAdmin() {
				c.Printf("Saving world state...")
				ack := make(chan error)
				c.Server.World.SaveWorldState <- SaveWorldStateMessage{Ack: ack}
				err := <-ack
				if err != nil {
					c.Printf("Error: %s\n", err.Error())
				} else {
					c.Printf("Complete\n")
				}
			} else {
				c.Printf("Not Authorized\n")
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "shutdown",
		Help: "Shutdown server (admin)",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if c.IsAdmin() {
				c.Printf("Shutting down the server...\n")
				c.Server.Shutdown <- true
			} else {
				c.Printf("Not Authorized\n")
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "create",
		Help: "Creates a new room or item. Usage: create <room|item> <name>",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if len(e.Args) > 1 {
				t := strings.TrimSpace(strings.ToLower(e.Args[0]))
				n := e.Args[1]
				if t == "room" {
					r := c.NewRoom(n)
					if r == nil {
						c.Println("Couldn't Create Room")
					} else {
						c.Printf("New Room Created: %s\n", r.String())
					}
				} else if t == "item" {
					i := c.NewItem(n)
					if i == nil {
						c.Println("Couldn't Create Item")
					} else {
						c.Printf("New Item Created: %s\n", i.String())
					}
				} else {
					c.Println(e.Cmd.HelpText())
				}
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "destroy",
		Help: "Destroys a room or item. Usage: destroy <room|item> <id>",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if len(e.Args) > 1 {
				t := strings.TrimSpace(strings.ToLower(e.Args[0]))
				id, err := ParseId(e.Args[1])
				if err != nil {
					c.Printf("Couldn't parse id: %s\n", e.Args[1])
					return
				}
				if t == "room" {
					r := c.DestroyRoom(id)
					if r == nil {
						c.Println("Couldn't Destroy Room")
					} else {
						c.Printf("Room Destroyed: %s\n", r.String())
					}
				} else if t == "item" {
					i := c.DestroyItem(id)
					if i == nil {
						c.Println("Couldn't Destroy Item")
					} else {
						c.Printf("Item Destroyed: %s\n", i.String())
					}
				} else {
					c.Println(e.Cmd.HelpText())
				}
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "list",
		Help: "List your rooms or items. Usage: list <rooms|items>",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if c.Player == nil {
				return
			}
			if len(e.Args) > 0 {
				t := strings.TrimSpace(strings.ToLower(e.Args[0]))
				if t == "rooms" {
					rooms := c.FindRoomsByOwner(c.Player.Id)
					c.ListRooms(rooms)
				} else if t == "items" {
					items := c.FindItemsByOwner(c.Player.Id)
					c.ListItems(items)
				} else {
					c.Println(e.Cmd.HelpText())
				}
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "inventory",
		Help: "List what you are carrying",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if c.Player == nil {
				return
			}
			items := c.FindItemsByLocation(Location{Id: c.Player.Id, Type: L_PLAYER})
			c.ListItems(items)
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "take",
		Help: "Pick up an item from the room you are in.  Usage: take <name or id>",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if len(e.Args) > 0 {
				c.Take(e.Args[0])
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "drop",
		Help: "Drop an item are carrying. Usage: drop <name or id>",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if len(e.Args) > 0 {
				c.Drop(e.Args[0])
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

}

func (c *Connection) IsAdmin() bool {
	return c != nil && c.Authenticated && c.Player != nil && c.Player.Admin
}

func (c *Connection) updateIdleTime() {
	c.LastActed = time.Now()
}

func (c *Connection) findPlayerConnectionByName(target string) (targetId IdType, targetName string) {
	t := strings.ToLower(target)
	for _, conn := range c.Server.Connections() {
		if conn.Authenticated && conn.Player != nil {
			n := strings.ToLower(conn.Player.Name)
			if t == n {
				targetId = conn.Id
				targetName = conn.Player.Name
				break
			}
		}
	}
	return
}

func (c *Connection) Say(target string, phrase string) {
	targetId, targetName := c.findPlayerConnectionByName(target)

	if target != "" {
		if targetName == "" {
			c.Printf("Couldn't find player %s\n", target)
			return
		}
	}

	// Send messages
	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		case conn.Id == c.Id:
			// Do Nothing
		case conn.Id == targetId && target != "":
			conn.Printf("%s says \"%s\" to you.\n", c.Player.Name, phrase)
		case target == "":
			conn.Printf("%s says \"%s\".\n", c.Player.Name, phrase)
		default:
			conn.Printf("%s says \"%s\" to %s.\n", c.Player.Name, phrase, targetName)
		}
	}
	if target == "" {
		c.Printf("You say \"%s\".\n", phrase)
	} else {
		c.Printf("You say \"%s\" to %s.\n", phrase, targetName)
	}

}

func (c *Connection) Whisper(target string, phrase string) {
	targetId, targetName := c.findPlayerConnectionByName(target)

	if targetName == "" {
		c.Printf("Couldn't find player %s\n", target)
		return
	}

	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		case conn.Id == c.Id:
			// Do Nothing
		case conn.Id == targetId:
			conn.Printf("%s whispers \"%s\".\n", c.Player.Name, phrase)
		default:
			conn.Printf("%s whispers to %s.\n", c.Player.Name, targetName)
		}
	}
	c.Printf("You whisper \"%s\" to %s.\n", phrase, targetName)
}

func (c *Connection) Emote(action string) {
	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		default:
			conn.Printf("%s %s.\n", c.Player.Name, action)
		}
	}
}

func (c *Connection) Look() {
	if c == nil || !c.Authenticated || c.Player == nil {
		return
	}
	loc := c.Player.Location
	s := ""
	switch loc.Type {
	case L_ROOM:
		r := c.FindRoomById(loc.Id)
		if r == nil {
			s = "You are lost.\n"
		} else {
			s = lookRoom(c, r)
		}
	default:
		// Not Yet Supported
		s = "You don't know where you are.\n"
	}
	c.Printf(s)
}

func lookRoom(c *Connection, r *Room) string {
	if c == nil || c.Player == nil || r == nil {
		return ""
	}
	p := c.Player
	playersHere := make([]string, 0)
	for _, conn := range c.Server.Connections() {
		p2 := conn.Player
		if conn.Authenticated && p2 != nil && p2.Id != p.Id {
			if p2.Location.Type == L_ROOM && p2.Location.Id == r.Id {
				playersHere = append(playersHere, p.Name)
			}
		}
	}
	itemsHere := c.FindItemsByLocation(c.Player.Location)
	s := r.String() + "\n"
	s += r.Desc + "\n"
	for _, item := range itemsHere {
		s += fmt.Sprintf("You see %s here.\n", item.Name)
	}
	for _, pName := range playersHere {
		s += fmt.Sprintf("You see %s here.\n", pName)
	}
	s += "\n"
	return s
}

const (
	h10 = "----------"
	h15 = "---------------"
	h20 = "--------------------"
	h30 = "------------------------------"
)

// TODO: Have the column widths auto-adjust to fit the data
func (c *Connection) Who() {
	s := "Players Currently Online:\n"
	f := "%10s %20s %20s %30s %15s\n"
	s += fmt.Sprintf(f, "Connection", "Player", "Location", "Connected", "Idle")
	s += fmt.Sprintf(f, h10, h20, h20, h30, h15)
	for _, conn := range c.Server.Connections() {
		playerName := "[Authenticating]"
		locName := "[UNKNOWN]"
		if conn.Authenticated && conn.Player != nil {
			playerName = conn.Player.String()
			locName = c.LocationName(conn.Player.Location)
		}

		connId := fmt.Sprintf("%10d", conn.Id)
		connected := conn.Connected.Format(time.RFC1123)
		idle := time.Since(conn.LastActed).String()

		s += fmt.Sprintf(f, connId, playerName, locName, connected, idle)

	}
	s += "\n"
	c.Printf(s)
}

func (c *Connection) ListRooms(rooms []*Room) {
	s := fmt.Sprintf("%10s %30s\n", "Id", "Room Name")
	s += fmt.Sprintf("%10s %30s\n", h10, h30)
	for _, r := range rooms {
		s += fmt.Sprintf("%10d %30s\n", r.Id, r.Name)
	}
	c.Println(s)
}

func (c *Connection) ListItems(items []*Item) {
	s := fmt.Sprintf("%10s %30s %30s\n", "Id", "Item Name", "Location")
	s += fmt.Sprintf("%10s %30s %30s\n", h10, h30, h30)
	for _, i := range items {
		s += fmt.Sprintf("%10d %30s %30s\n", i.Id, i.Name, c.LocationName(i.Location))
	}
	c.Println(s)
}

func (c *Connection) Take(itemName string) {
	if c.Player == nil {
		return
	}
	item, foundItems := c.FindLocalItem(c.Player.Location, itemName)
	if foundItems != nil {
		// Multiple items found
		c.Printf("Which item did you mean?\n")
		c.ListItems(foundItems)
	} else if item != nil {
		// Single item found
		item.Location = Location{Id: c.Player.Id, Type: L_PLAYER}
		c.Emote(fmt.Sprintf("picks up %s", item.Name))
	} else {
		// No items found
		c.Printf("That item is not here.\n")
	}
}

func (c *Connection) Drop(itemName string) {
	if c.Player == nil {
		return
	}
	item, foundItems := c.FindLocalItem(Location{Id: c.Player.Id, Type: L_PLAYER}, itemName)
	if foundItems != nil {
		// Multiple items found
		c.Printf("Which item did you mean?\n")
		c.ListItems(foundItems)
	} else if item != nil {
		// Single item found
		item.Location = c.Player.Location
		c.Emote(fmt.Sprintf("drops %s", item.Name))
	} else {
		// No items found
		c.Printf("You don't have that item.\n")
	}
}
