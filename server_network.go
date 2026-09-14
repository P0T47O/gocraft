package main

import (
	"fmt"
	"net"
	"time"
)

// ListenTCP establishes readiness synchronously, including an OS-assigned local port.
func (s *Server) ListenTCP(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.Listener = ln
	return nil
}

func (s *Server) StartTCP(addr string) error {
	if err := s.ListenTCP(addr); err != nil {
		s.World.Close()
		return err
	}
	s.ServeTCP()
	return nil
}

// ServeTCP runs after ListenTCP, and returns only after all sockets/workers close.
func (s *Server) ServeTCP() {
	ln := s.Listener
	fmt.Printf("Server listening on %s\n", ln.Addr())
	s.networkWG.Add(1)
	go func() {
		defer s.networkWG.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				s.Stop()
				return
			}
			s.ClientsMu.Lock()
			select {
			case <-s.Shutdown:
				s.ClientsMu.Unlock()
				conn.Close()
				return
			default:
			}
			s.connections[conn] = true
			s.networkWG.Add(1)
			s.ClientsMu.Unlock()
			go func() {
				defer s.networkWG.Done()
				defer func() {
					conn.Close()
					s.ClientsMu.Lock()
					delete(s.connections, conn)
					s.ClientsMu.Unlock()
				}()
				s.handleNewConnection(conn)
			}()
		}
	}()
	s.Start()
}

func (c *ClientConnection) close() {
	c.closeOnce.Do(func() {
		if c.done != nil {
			close(c.done)
		}
		if c.Conn != nil {
			c.Conn.Close()
		}
	})
}

// Critical messages never block the world loop. A saturated peer reconnects
// instead of silently losing authoritative inventory/block changes.
func (c *ClientConnection) enqueue(p Packet) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.Send <- p:
		return true
	default:
		c.close()
		return false
	}
}

func (s *Server) handleNewConnection(conn net.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	pkt, err := ReadPacket(conn)
	if err != nil {
		return
	}
	login, ok := pkt.(*PacketLogin)
	if !ok || login.ProtocolVersion != protocolVersion {
		_ = WritePacket(conn, &PacketChat{Message: "Protocol mismatch: update both client and server."})
		conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})
	name := login.Username
	cc := &ClientConnection{Name: name, Conn: conn, Send: make(chan Packet, 128), KnownChunks: make(map[chunkKey]bool), done: make(chan struct{})}
	s.ClientsMu.Lock()
	if old := s.Clients[name]; old != nil {
		old.close()
	}
	s.Clients[name] = cc
	s.ClientsMu.Unlock()
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		defer cc.close()
		for {
			select {
			case <-cc.done:
				return
			case p := <-cc.Send:
				if err := WritePacket(conn, p); err != nil {
					return
				}
			}
		}
	}()
	defer func() {
		cc.close()
		<-writerDone
		s.ClientsMu.Lock()
		// A previous connection must not remove its replacement on rapid rejoin.
		if s.Clients[name] == cc {
			delete(s.Clients, name)
		}
		s.ClientsMu.Unlock()
	}()
	for {
		select {
		case s.PacketCh <- PacketWrapper{Packet: pkt, From: name, Connection: cc}:
		case <-s.Shutdown:
			return
		case <-cc.done:
			return
		}
		pkt, err = ReadPacket(conn)
		if err != nil {
			return
		}
	}
}

func (s *Server) Broadcast(p Packet) {
	s.ClientsMu.RLock()
	defer s.ClientsMu.RUnlock()
	for _, c := range s.Clients {
		// Non-blocking for global broadcast to prevent one laggy client from stalling server
		select {
		case c.Send <- p:
		default:
			// fmt.Println("Dropped broadcast packet to", c.Name)
		}
	}
}

func (s *Server) BroadcastTo(name string, p Packet) {
	s.ClientsMu.RLock()
	defer s.ClientsMu.RUnlock()
	if c, ok := s.Clients[name]; ok {
		c.enqueue(p)
	}
}
