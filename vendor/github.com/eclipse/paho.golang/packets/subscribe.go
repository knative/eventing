package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

// Subscribe is the Variable Header definition for a Subscribe control packet
type Subscribe struct {
	Properties    *Properties
	Subscriptions []SubOptions
	PacketID      uint16
}

func (s *Subscribe) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "SUBSCRIBE: PacketID:%d Subscriptions:\n", s.PacketID)
	for _, o := range s.Subscriptions {
		fmt.Fprintf(&b, "\t%s: QOS:%d RetainHandling:%X NoLocal:%t RetainAsPublished:%t\n", o.Topic, o.QoS, o.RetainHandling, o.NoLocal, o.RetainAsPublished)
	}
	fmt.Fprintf(&b, "Properties:\n%s", s.Properties)

	return b.String()
}

// SetIdentifier sets the packet identifier
func (s *Subscribe) SetIdentifier(packetID uint16) {
	s.PacketID = packetID
}

// Type returns the current packet type
func (s *Subscribe) Type() byte {
	return SUBSCRIBE
}

// SubOptions is the struct representing the options for a subscription
type SubOptions struct {
	Topic             string
	QoS               byte
	RetainHandling    byte
	NoLocal           bool
	RetainAsPublished bool
}

// Pack is the implementation of the interface required function for a packet
// Note that this does not pack the topic
func (s *SubOptions) Pack() byte {
	var ret byte
	ret |= s.QoS & 0x03
	if s.NoLocal {
		ret |= 1 << 2
	}
	if s.RetainAsPublished {
		ret |= 1 << 3
	}
	ret |= (s.RetainHandling << 4) & 0x30

	return ret
}

// Unpack is the implementation of the interface required function for a packet
// Note that this does not unpack the topic
func (s *SubOptions) Unpack(r *bytes.Buffer) error {
	b, err := r.ReadByte()
	if err != nil {
		return err
	}

	s.QoS = b & 0x03
	s.NoLocal = b&(1<<2) != 0
	s.RetainAsPublished = b&(1<<3) != 0
	s.RetainHandling = 3 & (b >> 4)

	return nil
}

// Unpack is the implementation of the interface required function for a packet
func (s *Subscribe) Unpack(r *bytes.Buffer) error {
	var err error
	s.PacketID, err = readUint16(r)
	if err != nil {
		return err
	}

	err = s.Properties.Unpack(r, SUBSCRIBE)
	if err != nil {
		return err
	}

	for r.Len() > 0 {
		var so SubOptions
		t, err := readString(r)
		if err != nil {
			return err
		}
		if err = so.Unpack(r); err != nil {
			return err
		}
		so.Topic = t
		s.Subscriptions = append(s.Subscriptions, so)
	}

	return nil
}

// Buffers is the implementation of the interface required function for a packet
func (s *Subscribe) Buffers() net.Buffers {
	var b bytes.Buffer
	writeUint16(s.PacketID, &b)
	var subs bytes.Buffer
	for _, o := range s.Subscriptions {
		writeString(o.Topic, &subs)
		subs.WriteByte(o.Pack())
	}
	idvp := s.Properties.Pack(SUBSCRIBE)
	propLen := encodeVBI(len(idvp))
	return net.Buffers{b.Bytes(), propLen, idvp, subs.Bytes()}
}

// WriteTo is the implementation of the interface required function for a packet
func (s *Subscribe) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: SUBSCRIBE, Flags: 2}}
	cp.Content = s

	return cp.WriteTo(w)
}
