package main

import (
	"fmt"
)

func (s *Server) SendTo(player string, p Packet) {
	s.ClientsMu.RLock()
	client, ok := s.Clients[player]
	s.ClientsMu.RUnlock()
	if ok {
		client.enqueue(p)
	}
}

func (s *Server) handleCommand(player string, cmd string) {
	// Simple command parser
	parts := make([]string, 0)
	current := ""
	for _, c := range cmd {
		if c == ' ' {
			if len(current) > 0 {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if len(current) > 0 {
		parts = append(parts, current)
	}

	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/tp":
		if !s.LocalCheats {
			s.SendTo(player, &PacketChat{Message: "Commands are disabled on this server."})
			return
		}
		if len(parts) < 4 {
			s.SendTo(player, &PacketChat{Message: "Usage: /tp x y z"})
			return
		}
		var x, y, z float64
		fmt.Sscanf(parts[1], "%f", &x)
		fmt.Sscanf(parts[2], "%f", &y)
		fmt.Sscanf(parts[3], "%f", &z)

		s.SendTo(player, &PacketPlayerMove{X: x, Y: y, Z: z, Yaw: 0, Pitch: 0})
		s.SendTo(player, &PacketChat{Message: fmt.Sprintf("Teleported to %.1f %.1f %.1f", x, y, z)})

	case "/give":
		if !s.LocalCheats {
			s.SendTo(player, &PacketChat{Message: "Commands are disabled on this server."})
			return
		}
		if len(parts) < 2 {
			s.SendTo(player, &PacketChat{Message: "Usage: /give id [count]"})
			return
		}
		var id int
		count := 64
		fmt.Sscanf(parts[1], "%d", &id)
		if len(parts) >= 3 {
			fmt.Sscanf(parts[2], "%d", &count)
		}

		s.World.entitiesMu.Lock()
		var pEnt *PlayerEntity
		for _, e := range s.World.entities {
			if e.GetUUID() == player {
				if pe, ok := e.(*PlayerEntity); ok {
					pEnt = pe
				}
				break
			}
		}

		if pEnt != nil {
			if id <= 0 || id > 255 || Items[id] == nil || count < 1 || count > 2304 {
				s.World.entitiesMu.Unlock()
				return
			}
			remaining := pEnt.Inventory.Add(int32(id), int32(count))
			s.SendInventory(pEnt)

			if remaining < int32(count) {
				s.SendTo(player, &PacketChat{Message: fmt.Sprintf("Given %d of block %d", int32(count)-remaining, id)})
			} else {
				s.SendTo(player, &PacketChat{Message: "Inventory full"})
			}
		} else {
			s.SendTo(player, &PacketChat{Message: "Player entity not found"})
		}
		s.World.entitiesMu.Unlock()

	default:
		s.SendTo(player, &PacketChat{Message: "Unknown command: " + parts[0]})
	}
}
