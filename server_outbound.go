package main

import "fmt"

// A short-lived socket stall must not turn a reliable gameplay transition
// into a disconnect. Keep a bounded overflow FIFO, retaining order across
// the channel and overflow. Sustained non-consuming peers remain bounded.
const serverReliableOverflowCapacity = 1024

func (c *ClientConnection) outboundReserve() int { return min(48, cap(c.Send)/2) }

func (c *ClientConnection) enqueueReliable(p Packet) bool {
	c.outboundMu.Lock()
	defer c.outboundMu.Unlock()
	select {
	case <-c.done:
		return false
	default:
	}
	if len(c.outboundPending) == 0 {
		select {
		case c.Send <- p:
			return true
		default:
		}
	}
	if len(c.outboundPending) < serverReliableOverflowCapacity {
		c.outboundPending = append(c.outboundPending, p)
		return true
	}
	fmt.Printf("Disconnecting %s: reliable outbound backlog saturated (%d queued + %d pending), packet=%d\n", c.Name, len(c.Send), len(c.outboundPending), p.ID())
	c.close()
	return false
}

// Snapshots are replaceable and may never consume the gameplay reservation,
// or overtake reliable messages already waiting in the overflow FIFO.
func (c *ClientConnection) enqueueSnapshot(p Packet) {
	c.outboundMu.Lock()
	defer c.outboundMu.Unlock()
	select {
	case <-c.done:
		return
	default:
	}
	if len(c.outboundPending) > 0 || cap(c.Send)-len(c.Send) <= c.outboundReserve() {
		return
	}
	select {
	case c.Send <- p:
	default:
	}
}

// Replay pairs (spawn followed by mob state) are admitted together. They
// pause behind gameplay backpressure rather than entering the overflow FIFO.
func (c *ClientConnection) enqueueReplay(packets []Packet) bool {
	c.outboundMu.Lock()
	defer c.outboundMu.Unlock()
	select {
	case <-c.done:
		return false
	default:
	}
	if len(c.outboundPending) > 0 || cap(c.Send)-len(c.Send)-c.outboundReserve() < len(packets) {
		return false
	}
	for _, p := range packets {
		c.Send <- p
	}
	return true
}

// Called by the sole socket writer after it removes a channel packet. The
// mutex prevents producers from inserting newer packets ahead of pending ones.
func (c *ClientConnection) refillOutbound() {
	c.outboundMu.Lock()
	defer c.outboundMu.Unlock()
	count := 0
	for count < len(c.outboundPending) {
		select {
		case c.Send <- c.outboundPending[count]:
			c.outboundPending[count] = nil
			count++
		default:
			c.outboundPending = c.outboundPending[count:]
			return
		}
	}
	c.outboundPending = nil
}
