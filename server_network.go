package main

import (
	"fmt"
	"net"
	"time"
)

const serverStreamQueueCapacity = 8

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

// Critical messages never block the world loop. Streaming uses a separate,
// shallow queue so a slow chunk consumer cannot occupy every slot needed by
// authoritative inventory/block/container transitions.
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
		fmt.Printf("Disconnecting %s: outbound queue saturated (%d/%d), packet=%d\n", c.Name, len(c.Send), cap(c.Send), p.ID())
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
	cc := &ClientConnection{
		Name:        name,
		Conn:        conn,
		Send:        make(chan Packet, 128),
		StreamSend:  make(chan Packet, serverStreamQueueCapacity),
		KnownChunks: make(map[chunkKey]bool),
		done:        make(chan struct{}),
	}
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

		write := func(p Packet) bool {
			start := time.Now()
			err := WritePacket(conn, p)
			if p.ID() == IDChunkData || p.ID() == IDChunkLight {
				perfMon.recordLoading(phaseNetworkWrite, start)
			}
			return err == nil
		}

		for {
			// Give already-queued authoritative traffic first refusal before
			// taking another large chunk snapshot.
			select {
			case <-cc.done:
				return
			case p := <-cc.Send:
				if !write(p) {
					return
				}
				continue
			default:
			}

			select {
			case <-cc.done:
				return
			case p := <-cc.Send:
				if !write(p) {
					return
				}
			case p := <-cc.StreamSend:
				if !write(p) {
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

	// Movement snapshots and chunk-unload hints are replaceable. A client also
	// garbage-collects far chunks locally, and the server clears KnownChunks when
	// it unloads one, so dropping an unload hint under backpressure is preferable
	// to disconnecting an otherwise healthy client. Authoritative inventory,
	// block, spawn/despawn, metadata, vitals and chat transitions still use the
	// reliable enqueue path below.
	bestEffort := p.ID() == IDPlayerMove || p.ID() == IDEntityMove || p.ID() == IDUnloadChunk || p.ID() == IDWorldTime || p.ID() == 0x1D
	for _, c := range s.Clients {
		if bestEffort {
			select {
			case c.Send <- p:
			default:
			}
			continue
		}
		c.enqueue(p)
	}
}

func (s *Server) BroadcastTo(name string, p Packet) {
	s.ClientsMu.RLock()
	defer s.ClientsMu.RUnlock()
	if c, ok := s.Clients[name]; ok {
		c.enqueue(p)
	}
}
