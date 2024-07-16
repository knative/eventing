package packets

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
)

// Connect is the Variable Header definition for a connect control packet
type Connect struct {
	WillMessage     []byte
	Password        []byte
	Username        string
	ProtocolName    string
	ClientID        string
	WillTopic       string
	Properties      *Properties
	WillProperties  *Properties
	KeepAlive       uint16
	ProtocolVersion byte
	WillQOS         byte
	PasswordFlag    bool
	UsernameFlag    bool
	WillRetain      bool
	WillFlag        bool
	CleanStart      bool
}

func (c *Connect) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "CONNECT: ProtocolName:%s ProtocolVersion:%d ClientID:%s KeepAlive:%d CleanStart:%t", c.ProtocolName, c.ProtocolVersion, c.ClientID, c.KeepAlive, c.CleanStart)
	if c.UsernameFlag {
		fmt.Fprintf(&b, " Username:%s", c.Username)
	}
	if c.PasswordFlag {
		fmt.Fprintf(&b, " Password:%s", c.Password)
	}
	fmt.Fprint(&b, "\n")
	if c.WillFlag {
		fmt.Fprintf(&b, " WillTopic:%s WillQOS:%d WillRetain:%t WillMessage:\n%s\n", c.WillTopic, c.WillQOS, c.WillRetain, c.WillMessage)
		if c.WillProperties != nil {
			fmt.Fprintf(&b, "WillProperties:\n%s", c.WillProperties)
		}
	}
	if c.Properties != nil {
		fmt.Fprintf(&b, "Properties:\n%s", c.Properties)
	}

	return b.String()
}

// PackFlags takes the Connect flags and packs them into the single byte
// representation used on the wire by MQTT
func (c *Connect) PackFlags() (f byte) {
	if c.UsernameFlag {
		f |= 0x01 << 7
	}
	if c.PasswordFlag {
		f |= 0x01 << 6
	}
	if c.WillFlag {
		f |= 0x01 << 2
		f |= c.WillQOS << 3
		if c.WillRetain {
			f |= 0x01 << 5
		}
	}
	if c.CleanStart {
		f |= 0x01 << 1
	}
	return
}

// UnpackFlags takes the wire byte representing the connect options flags
// and fills out the appropriate variables in the struct
func (c *Connect) UnpackFlags(b byte) {
	c.CleanStart = 1&(b>>1) > 0
	c.WillFlag = 1&(b>>2) > 0
	c.WillQOS = 3 & (b >> 3)
	c.WillRetain = 1&(b>>5) > 0
	c.PasswordFlag = 1&(b>>6) > 0
	c.UsernameFlag = 1&(b>>7) > 0
}

//Unpack is the implementation of the interface required function for a packet
func (c *Connect) Unpack(r *bytes.Buffer) error {
	var err error

	if c.ProtocolName, err = readString(r); err != nil {
		return err
	}

	if c.ProtocolVersion, err = r.ReadByte(); err != nil {
		return err
	}

	flags, err := r.ReadByte()
	if err != nil {
		return err
	}
	c.UnpackFlags(flags)

	if c.KeepAlive, err = readUint16(r); err != nil {
		return err
	}

	err = c.Properties.Unpack(r, CONNECT)
	if err != nil {
		return err
	}

	c.ClientID, err = readString(r)
	if err != nil {
		return err
	}

	if c.WillFlag {
		c.WillProperties = &Properties{}
		err = c.WillProperties.Unpack(r, CONNECT)
		if err != nil {
			return err
		}
		c.WillTopic, err = readString(r)
		if err != nil {
			return err
		}
		c.WillMessage, err = readBinary(r)
		if err != nil {
			return err
		}
	}

	if c.UsernameFlag {
		c.Username, err = readString(r)
		if err != nil {
			return err
		}
	}

	if c.PasswordFlag {
		c.Password, err = readBinary(r)
		if err != nil {
			return err
		}
	}

	return nil
}

// Buffers is the implementation of the interface required function for a packet
func (c *Connect) Buffers() net.Buffers {
	var cp bytes.Buffer

	writeString(c.ProtocolName, &cp)
	cp.WriteByte(c.ProtocolVersion)
	cp.WriteByte(c.PackFlags())
	writeUint16(c.KeepAlive, &cp)
	idvp := c.Properties.Pack(CONNECT)
	encodeVBIdirect(len(idvp), &cp)
	cp.Write(idvp)

	writeString(c.ClientID, &cp)
	if c.WillFlag {
		willIdvp := c.WillProperties.Pack(CONNECT)
		encodeVBIdirect(len(willIdvp), &cp)
		cp.Write(willIdvp)
		writeString(c.WillTopic, &cp)
		writeBinary(c.WillMessage, &cp)
	}
	if c.UsernameFlag {
		writeString(c.Username, &cp)
	}
	if c.PasswordFlag {
		writeBinary(c.Password, &cp)
	}

	return net.Buffers{cp.Bytes()}
}

// WriteTo is the implementation of the interface required function for a packet
func (c *Connect) WriteTo(w io.Writer) (int64, error) {
	cp := &ControlPacket{FixedHeader: FixedHeader{Type: CONNECT}}
	cp.Content = c

	return cp.WriteTo(w)
}
