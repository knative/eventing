package paho

import "github.com/eclipse/paho.golang/packets"

type (
	// Suback is a representation of an MQTT suback packet
	Suback struct {
		Properties *SubackProperties
		Reasons    []byte
	}

	// SubackProperties is a struct of the properties that can be set
	// for a Suback packet
	SubackProperties struct {
		ReasonString string
		User         UserProperties
	}
)

// Packet returns a packets library Suback from the paho Suback
// on which it is called
func (s *Suback) Packet() *packets.Suback {
	return &packets.Suback{
		Reasons: s.Reasons,
		Properties: &packets.Properties{
			User: s.Properties.User.ToPacketProperties(),
		},
	}
}

// SubackFromPacketSuback takes a packets library Suback and
// returns a paho library Suback
func SubackFromPacketSuback(s *packets.Suback) *Suback {
	return &Suback{
		Reasons: s.Reasons,
		Properties: &SubackProperties{
			ReasonString: s.Properties.ReasonString,
			User:         UserPropertiesFromPacketUser(s.Properties.User),
		},
	}
}
