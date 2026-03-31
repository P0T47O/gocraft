package main

import (
	"fmt"
	"net"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Client represents the diverse state needed to communicate with the server.
type Client struct {
	Conn          net.Conn
	Name          string
	Incoming      chan Packet // Channel to receive packets from server
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
		Incoming: make(chan Packet, 4096),
	}

	// Reader Loop
	go func() {
		defer close(c.Incoming)
		for {
			p, err := ReadPacket(conn)
			if err != nil {
				fmt.Printf("Disconnected from server: %v\n", err)
				break
			}
			c.Incoming <- p
		}
	}()

	// Send Login
	login := &PacketLogin{
		ProtocolVersion: 1,
		Username:        name,
	}
	c.Send(login)

	return c, nil
}

func (c *Client) Send(p Packet) {
	err := WritePacket(c.Conn, p)
	if err != nil {
		fmt.Printf("Send error: %v\n", err)
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
