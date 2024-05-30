package paho

import "github.com/eclipse/paho.golang/packets"

type (
	// Disconnect is a representation of the MQTT Disconnect packet
	Disconnect struct {
		Properties *DisconnectProperties
		ReasonCode byte
	}

	// DisconnectProperties is a struct of the properties that can be set
	// for a Disconnect packet
	DisconnectProperties struct {
		ServerReference       string
		ReasonString          string
		SessionExpiryInterval *uint32
		User                  UserProperties
	}
)

// InitProperties is a function that takes a lower level
// Properties struct and completes the properties of the Disconnect on
// which it is called
func (d *Disconnect) InitProperties(p *packets.Properties) {
	d.Properties = &DisconnectProperties{
		SessionExpiryInterval: p.SessionExpiryInterval,
		ServerReference:       p.ServerReference,
		ReasonString:          p.ReasonString,
		User:                  UserPropertiesFromPacketUser(p.User),
	}
}

// DisconnectFromPacketDisconnect takes a packets library Disconnect and
// returns a paho library Disconnect
func DisconnectFromPacketDisconnect(p *packets.Disconnect) *Disconnect {
	v := &Disconnect{ReasonCode: p.ReasonCode}
	v.InitProperties(p.Properties)

	return v
}

// Packet returns a packets library Disconnect from the paho Disconnect
// on which it is called
func (d *Disconnect) Packet() *packets.Disconnect {
	v := &packets.Disconnect{ReasonCode: d.ReasonCode}

	if d.Properties != nil {
		v.Properties = &packets.Properties{
			SessionExpiryInterval: d.Properties.SessionExpiryInterval,
			ServerReference:       d.Properties.ServerReference,
			ReasonString:          d.Properties.ReasonString,
			User:                  d.Properties.User.ToPacketProperties(),
		}
	}

	return v
}
