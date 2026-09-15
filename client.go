package main

import (
	"fmt"
	"net"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Client represents the diverse state needed to communicate with the server.
type Client struct {
	done          chan struct{}
	closeOnce     sync.Once
	reader        sync.WaitGroup
	writer        sync.WaitGroup
	Conn          net.Conn
	Name          string
	Incoming      chan Packet // Channel to receive packets from server
	Outgoing      chan Packet // Bounded queue drained by the socket writer.
	LastSentX     float64
	LastSentY     float64
	LastSentZ     float64
	LastSentYaw   float32
	LastSentPitch float32
	Inventory     Inventory
}

func ConnectTCP(addr string, name string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	c := &Client{
		Conn:     conn,
		Name:     name,
		Incoming: make(chan Packet, 128), // Bounded snapshots; TCP supplies backpressure.
		Outgoing: make(chan Packet, 128), // Keep socket stalls off the render/update thread.
		done:     make(chan struct{}),
	}

	// Writer loop. Closing Conn unblocks a stalled WritePacket during shutdown.
	c.writer.Add(1)
	go func() {
		defer c.writer.Done()
		for {
			select {
			case <-c.done:
				return
			case p := <-c.Outgoing:
				if err := WritePacket(conn, p); err != nil {
					c.closeOnce.Do(func() {
						close(c.done)
						_ = conn.Close()
					})
					fmt.Printf("Send error: %v\n", err)
					return
				}
			}
		}
	}()

	// Reader Loop
	c.reader.Add(1)
	go func() {
		defer c.reader.Done()
		defer close(c.Incoming)
		for {
			p, err := ReadPacket(conn)
			if err != nil {
				c.closeOnce.Do(func() {
					close(c.done)
					_ = conn.Close()
				})
				fmt.Printf("Disconnected from server: %v\n", err)
				return
			}
			select {
			case c.Incoming <- p:
			case <-c.done:
				return
			}
		}
	}()

	// Send Login
	login := &PacketLogin{
		ProtocolVersion: protocolVersion,
		Username:        name,
	}
	c.Send(login)

	return c, nil
}

func (c *Client) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.Conn.Close()
	})
	c.reader.Wait()
	c.writer.Wait()
}

func (c *Client) Send(p Packet) {
	select {
	case <-c.done:
		return
	default:
	}

	// Player movement is a latest-state snapshot. If the writer is temporarily
	// behind, dropping one movement sample is preferable to blocking a frame.
	if p.ID() == IDPlayerMove {
		select {
		case c.Outgoing <- p:
		case <-c.done:
		default:
		}
		return
	}

	// Gameplay actions are authoritative requests and must not disappear
	// silently. A client that cannot enqueue them is already too far behind to
	// remain synchronized, so close it instead of blocking the render thread.
	select {
	case c.Outgoing <- p:
	case <-c.done:
	default:
		fmt.Printf("Disconnecting client: outbound queue saturated (%d/%d), packet=%d\n", len(c.Outgoing), cap(c.Outgoing), p.ID())
		c.closeOnce.Do(func() {
			close(c.done)
			_ = c.Conn.Close()
		})
	}
}

func (c *Client) Update(camera *rl.Camera3D, input *InputState) {
	// Send position/rotation if either position or rotation changed
	distSq := (float64(camera.Position.X)-c.LastSentX)*(float64(camera.Position.X)-c.LastSentX) +
		(float64(camera.Position.Y)-c.LastSentY)*(float64(camera.Position.Y)-c.LastSentY) +
		(float64(camera.Position.Z)-c.LastSentZ)*(float64(camera.Position.Z)-c.LastSentZ)

	// Also check rotation change
	yawDiff := input.Yaw - c.LastSentYaw
	pitchDiff := input.Pitch - c.LastSentPitch
	rotChanged := (yawDiff*yawDiff + pitchDiff*pitchDiff) > 0.01

	if distSq > 0.01 || rotChanged {
		c.Send(&PacketPlayerMove{
			X:     float64(camera.Position.X),
			Y:     float64(camera.Position.Y),
			Z:     float64(camera.Position.Z),
			Yaw:   input.Yaw,
			Pitch: input.Pitch,
		})
		c.LastSentX = float64(camera.Position.X)
		c.LastSentY = float64(camera.Position.Y)
		c.LastSentZ = float64(camera.Position.Z)
		c.LastSentYaw = input.Yaw
		c.LastSentPitch = input.Pitch
	}
}
