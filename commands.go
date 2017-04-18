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
	"strings"
	"time"

	"github.com/abiosoft/ishell"
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
			if c.Player == nil {
				return
			}
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
				c.Say(target, phrase, &c.Player.Location)
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "whisper",
		Help: "Whisper something to the somebody else. Usage: whisper <player> <message>",
		Func: func(e *ishell.Context) {
			if c.Player == nil {
				return
			}
			if len(e.Args) > 1 {
				c.updateIdleTime()
				target := e.Args[0]
				phrase := e.Args[1]
				c.Log("Executing Whisper: %s - %s", target, phrase)
				c.Whisper(target, phrase, &c.Player.Location)
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "emote",
		Help: "Do something. Usage: emote <action>",
		Func: func(e *ishell.Context) {
			if c.Player == nil {
				return
			}
			if len(e.Args) > 0 {
				c.updateIdleTime()
				action := e.Args[0]
				c.Log("Executing Emote: %s", action)
				c.Emote(action, &c.Player.Location)
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

	shell.AddCmd(&ishell.Cmd{
		Name: "look",
		Help: "Look around. Usage: look [target]",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			target := ""
			if len(e.Args) > 0 {
				target = e.Args[0]
			}
			c.Look(target)
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
		Help: "Creates a new room or item. Usage: create <room|item> <name> [description]",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if len(e.Args) > 1 {
				t := strings.TrimSpace(strings.ToLower(e.Args[0]))
				n := e.Args[1]
				d := ""
				if len(e.Args) > 2 {
					d = e.Args[2]
				}
				if t == "room" {
					r := c.NewRoom(n, d)
					if r == nil {
						c.Println("Couldn't Create Room")
					} else {
						c.Printf("New Room Created: %s\n", r.String())
					}
				} else if t == "item" {
					i := c.NewItem(n, d)
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
				id, err := ParseID(e.Args[1])
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
		Help: "List your rooms or items. Usage: list <rooms|items|players>",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if c.Player == nil {
				return
			}
			if len(e.Args) > 0 {
				t := strings.TrimSpace(strings.ToLower(e.Args[0]))
				all := false
				if t == "all" && c.IsAdmin() && len(e.Args) > 1 {
					all = true
					t = strings.TrimSpace(strings.ToLower(e.Args[1]))
				}
				switch t {
				case "rooms":
					var rooms []*Room
					if all {
						rooms = c.FindAllRooms()
					} else {
						rooms = c.FindRoomsByOwner(c.Player.ID)
					}
					c.ListRooms(rooms)
				case "items":
					var items []*Item
					if all {
						items = c.FindAllItems()
					} else {
						items = c.FindItemsByOwner(c.Player.ID)
					}
					c.ListItems(items)
				case "players":
					var players []*Player
					if all {
						players = c.FindAllPlayers()
					} else {
						players = c.FindOnlinePlayersByLocation(nil)
					}
					c.ListPlayers(players)
				default:
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
			items := c.FindItemsByLocation(Location{ID: c.Player.ID, Type: LocationPlayer})
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

	shell.AddCmd(&ishell.Cmd{
		Name: "go",
		Help: "Go somewhere.  Usage: go <direction>",
		Func: func(e *ishell.Context) {
			c.updateIdleTime()
			if len(e.Args) > 0 {
				c.Go(e.Args[0])
			} else {
				c.Println(e.Cmd.HelpText())
			}
		},
	})

}

// IsAdmin returns true if the player is an admin.
func (c *Connection) IsAdmin() bool {
	return c != nil && c.Authenticated && c.Player != nil && c.Player.Admin
}

func (c *Connection) updateIdleTime() {
	c.LastActed = time.Now()
	if c.Player != nil {
		c.Player.LastActed = time.Now()
	}
}

func (c *Connection) findPlayerConnectionByName(target string) (targetID IDType, targetName string, loc Location) {
	t := strings.ToLower(target)
	for _, conn := range c.Server.Connections() {
		if conn.Authenticated && conn.Player != nil {
			n := strings.ToLower(conn.Player.Name)
			if t == n {
				targetID = conn.ID
				targetName = conn.Player.Name
				loc = conn.Player.Location
				break
			}
		}
	}
	return
}

// Say executes the "say" command for the given player.
func (c *Connection) Say(target string, phrase string, loc *Location) {
	targetID, targetName, targetLoc := c.findPlayerConnectionByName(target)

	if target != "" {
		if targetName == "" {
			c.Printf("Couldn't find player %s\n", target)
			return
		}

		if loc != nil && *loc != targetLoc {
			c.Printf("That player is not here.\n")
			return
		}
	}

	// Send messages
	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		case conn.ID == c.ID:
			// Do Nothing
		case conn.ID == targetID && target != "":
			conn.Printf("%s says \"%s\" to you.\n", c.Player.Name, phrase)
		case target == "" && conn.InLocation(loc):
			conn.Printf("%s says \"%s\".\n", c.Player.Name, phrase)
		case conn.InLocation(loc):
			conn.Printf("%s says \"%s\" to %s.\n", c.Player.Name, phrase, targetName)
		}
	}
	if target == "" {
		c.Printf("You say \"%s\".\n", phrase)
	} else {
		c.Printf("You say \"%s\" to %s.\n", phrase, targetName)
	}

}

// Whisper executes the "whisper" command for the given player.
func (c *Connection) Whisper(target string, phrase string, loc *Location) {
	targetID, targetName, targetLoc := c.findPlayerConnectionByName(target)

	if targetName == "" {
		c.Printf("Couldn't find player %s\n", target)
		return
	}

	if loc != nil && *loc != targetLoc {
		c.Printf("That player is not here.\n")
		return
	}

	for _, conn := range c.Server.Connections() {
		switch {
		case !conn.Authenticated:
			// Do Nothing
		case conn.ID == c.ID:
			// Do Nothing
		case conn.ID == targetID:
			conn.Printf("%s whispers \"%s\".\n", c.Player.Name, phrase)
		case conn.InLocation(loc):
			conn.Printf("%s whispers to %s.\n", c.Player.Name, targetName)
		}
	}
	c.Printf("You whisper \"%s\" to %s.\n", phrase, targetName)
}

// Emote executes the "emote" command for the given player.
// It can also be used by other commands to say that the player did something.
func (c *Connection) Emote(action string, loc *Location) {
	c.LocationPrintf(loc, "%s %s.\n", c.Player.Name, action)
}

// Look executes the "look" command for the given player.
func (c *Connection) Look(target string) {
	if c == nil || !c.Authenticated || c.Player == nil {
		return
	}
	loc := c.Player.Location
	if target == "" {
		// Look at the current location
		s := ""
		switch loc.Type {
		case LocationRoom:
			r := c.FindRoomByID(loc.ID)
			if r == nil {
				s = "You are lost.\n"
			} else {
				s = c.lookRoom(r)
			}
		default:
			// Not Yet Supported
			s = "You don't know where you are.\n"
		}
		c.Printf(s)
	} else {
		// Look at a given target
		roomItem, foundRoomItems := c.FindLocalThing(loc, target, true)
		playerItem, foundPlayerItems := c.FindLocalThing(Location{ID: c.Player.ID, Type: LocationPlayer}, target, true)

		allItems := make([]fmt.Stringer, 0)
		if roomItem != nil {
			allItems = append(allItems, roomItem)
		}
		if playerItem != nil {
			allItems = append(allItems, playerItem)
		}
		if foundRoomItems != nil {
			allItems = append(allItems, foundRoomItems...)
		}
		if foundPlayerItems != nil {
			allItems = append(allItems, foundPlayerItems...)
		}

		if len(allItems) > 1 {
			// Multiple items found
			c.Printf("Which thing did you mean?\n")
			for _, i := range allItems {
				c.Printf("%s\n", i)
			}
		} else if len(allItems) == 1 {
			// Single item found
			c.Printf(c.lookThing(allItems[0]))
		} else {
			// No items found
			c.Printf("That item is not here.\n")
		}

	}
}

func (c *Connection) lookThing(t interface{}) string {
	if c != nil && t != nil {
		i, ok := t.(*Item)
		if ok {
			return c.lookItem(i)
		}
		p, ok := t.(*Player)
		if ok {
			return c.lookPlayer(p)
		}
		e, ok := t.(*Exit)
		if ok {
			return c.lookExit(e)
		}
		r, ok := t.(*Room)
		if ok {
			return c.lookRoom(r)
		}
	}
	return ""
}

func (c *Connection) lookRoom(r *Room) string {
	if c == nil || c.Player == nil || !c.Authenticated || r == nil {
		return ""
	}
	p := c.Player

	loc := Location{ID: r.ID, Type: LocationRoom}

	s := r.String() + "\n"
	s += r.Description + "\n"
	// Exits
	for _, exit := range r.Exits {
		s += fmt.Sprintf("%s [%s]\n", exit.Description, exit.Name)
	}

	// Items
	for _, item := range c.FindItemsByLocation(loc) {
		if item != nil {
			s += fmt.Sprintf("You see %s here.\n", item.Name)
		}
	}

	// Players
	for _, player := range c.FindOnlinePlayersByLocation(&loc) {
		if player != nil && p.ID != player.ID {
			s += fmt.Sprintf("You see %s here.\n", player.Name)
		}
	}

	s += "\n"
	return s
}

func (c *Connection) lookItem(i *Item) string {
	if c == nil || c.Player == nil || !c.Authenticated || i == nil {
		return ""
	}
	return fmt.Sprintf("%s\n", i.Description)
}

func (c *Connection) lookPlayer(p *Player) string {
	if c == nil || c.Player == nil || !c.Authenticated || p == nil {
		return ""
	}
	return fmt.Sprintf("%s\n", p.Description)
}

func (c *Connection) lookExit(e *Exit) string {
	if c == nil || c.Player == nil || !c.Authenticated || e == nil {
		return ""
	}
	return fmt.Sprintf("%s\n", e.LongDescription)
}

const (
	h5  = "-----"
	h10 = "----------"
	h15 = "---------------"
	h20 = "--------------------"
	h30 = "------------------------------"
)

// Who shows a list of the currently logged in players.
// TODO: Have the column widths auto-adjust to fit the data
func (c *Connection) Who() {
	s := "Players Currently Online:\n"
	f := "%10s %20s %20s %30s %15s %5s\n"
	s += fmt.Sprintf(f, "Connection", "Player", "Location", "Connected", "Idle", "Admin")
	s += fmt.Sprintf(f, h10, h20, h20, h30, h15, h5)
	for _, conn := range c.Server.Connections() {
		playerName := "[Authenticating]"
		locName := "[UNKNOWN]"
		admin := "No"
		if conn.Authenticated && conn.Player != nil {
			playerName = conn.Player.String()
			locName = c.LocationName(conn.Player.Location)
			if conn.Player.Admin {
				admin = "Yes"
			}
		}

		connID := fmt.Sprintf("%10d", conn.ID)
		connected := conn.Connected.Format(time.RFC1123)
		idle := time.Since(conn.LastActed).String()

		s += fmt.Sprintf(f, connID, playerName, locName, connected, idle, admin)

	}
	s += "\n"
	c.Printf(s)
}

// ListPlayers shows a list of players.
func (c *Connection) ListPlayers(players []*Player) {
	s := "Players:\n"
	f := "%20s %20s %30s %5s\n"
	s += fmt.Sprintf(f, "Player", "Location", "Last Active", "Admin")
	s += fmt.Sprintf(f, h20, h20, h30, h5)
	for _, p := range players {
		playerName := "[Authenticating]"
		locName := "[UNKNOWN]"
		active := "[NEVER]"
		admin := "No"
		if p != nil {
			playerName = p.String()
			locName = c.LocationName(p.Location)
			if p.Admin {
				admin = "Yes"
			}
			active = fmt.Sprintf("%s ago", time.Since(p.LastActed).String())
		}
		s += fmt.Sprintf(f, playerName, locName, active, admin)

	}
	s += "\n"
	c.Printf(s)
}


// ListRooms displays a list of the given rooms.
func (c *Connection) ListRooms(rooms []*Room) {
	s := fmt.Sprintf("%10s %30s\n", "ID", "Room Name")
	s += fmt.Sprintf("%10s %30s\n", h10, h30)
	for _, r := range rooms {
		s += fmt.Sprintf("%10s %30s\n", r.ID, r.Name)
	}
	c.Println(s)
}

// ListItems displays a list of the given items.
func (c *Connection) ListItems(items []*Item) {
	s := fmt.Sprintf("%10s %30s %30s\n", "ID", "Item Name", "Location")
	s += fmt.Sprintf("%10s %30s %30s\n", h10, h30, h30)
	for _, i := range items {
		s += fmt.Sprintf("%10s %30s %30s\n", i.ID, i.Name, c.LocationName(i.Location))
	}
	c.Println(s)
}

// Take executes the "take" command and moves an item into the player's inventory.
func (c *Connection) Take(itemName string) {
	if c.Player == nil {
		return
	}
	foundOne, foundMany := c.FindLocalThing(c.Player.Location, itemName, false)
	if foundMany != nil && len(foundMany) > 0 {
		// Multiple items found
		c.Printf("Which item did you mean?\n")
		for _, t := range foundMany {
			c.Printf("%s\n", t)
		}
	} else if foundOne != nil {
		// Single item found
		item, ok := foundOne.(*Item)
		if ok {
			item.Location = Location{ID: c.Player.ID, Type: LocationPlayer}
			c.Emote(fmt.Sprintf("picks up %s", item.Name), &c.Player.Location)
		} else {
			c.Printf("You can't take that.\n")
		}
	} else {
		// No items found
		c.Printf("That item is not here.\n")
	}
}

// Drop executes the "drop" command and moves an item out of the player's inventory.
func (c *Connection) Drop(itemName string) {
	if c.Player == nil {
		return
	}
	foundOne, foundMany := c.FindLocalThing(Location{ID: c.Player.ID, Type: LocationPlayer}, itemName, false)
	if foundMany != nil && len(foundMany) > 0 {
		// Multiple items found
		c.Printf("Which item did you mean?\n")
		for _, t := range foundMany {
			c.Printf("%s\n", t)
		}
	} else if foundOne != nil {
		// Single item found
		item, ok := foundOne.(*Item)
		if ok {
			item.Location = c.Player.Location
			c.Emote(fmt.Sprintf("drops %s", item.Name), &c.Player.Location)
		} else {
			c.Printf("You can't drop that.\n")
		}
	} else {
		// No items found
		c.Printf("You don't have that item.\n")
	}
}

// Go executes the "go" command and moves a player to another room.
func (c *Connection) Go(target string) {
	if c == nil || !c.Authenticated || c.Player == nil {
		return
	}
	t := strings.TrimSpace(strings.ToLower(target))
	switch c.Player.Location.Type {
	case LocationRoom:
		r := c.FindRoomByID(c.Player.Location.ID)
		if r == nil {
			c.Printf("You're Lost!\n")
			return
		}
		for _, e := range r.Exits {
			if strings.ToLower(e.Name) == t {
				dest := c.FindRoomByID(e.Destination)
				if dest == nil {
					c.Printf("That doesn't seem to go anywhere.\n")
				}
				// TODO: Handle locks here!
				c.Move(Location{ID: dest.ID, Type: LocationRoom}, e.LeaveMessage, e.ArriveMessage)
			}
		}
	default:
		c.Printf("You're not in a room!\n")
		return
	}
}

// Move transports a player to another location.
// leaveMessage should contain "%s" for the player's name.
// arriveMessage should contain "%s" for the player's name.
func (c *Connection) Move(destination Location, leaveMessage string, arriveMessage string) {
	if c == nil || !c.Authenticated || c.Player == nil {
		return
	}
	c.LocationPrintf(&c.Player.Location, leaveMessage+"\n", c.Player.Name)
	c.Player.Location = destination
	c.Look("")
	c.LocationPrintf(&destination, arriveMessage+"\n", c.Player.Name)
}
