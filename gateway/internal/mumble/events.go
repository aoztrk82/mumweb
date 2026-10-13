package mumble

import pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"

// Event is published on Client.Events after the handshake.
type Event interface{ isEvent() }

// ChannelEvent carries the merged channel state after a ChannelState update.
type ChannelEvent struct{ Channel Channel }

// ChannelRemoveEvent reports a deleted channel.
type ChannelRemoveEvent struct{ ID uint32 }

// UserEvent carries the merged user state after a UserState update.
type UserEvent struct {
	User  User
	Actor uint32 // session that caused the change (0 if unset)
}

// UserRemoveEvent reports a user leaving, being kicked or banned.
// If Session is our own, the server is about to close the connection.
type UserRemoveEvent struct {
	Session uint32
	Actor   uint32
	Reason  string
	Ban     bool
}

// TextEvent is a received (HTML) text message.
type TextEvent struct {
	Actor    uint32
	Sessions []uint32
	Channels []uint32
	Trees    []uint32
	Message  string
}

// DeniedEvent is a PermissionDenied message.
type DeniedEvent struct {
	Type       pb.PermissionDenied_DenyType
	Reason     string
	Permission uint32
	ChannelID  uint32
	Session    uint32
	Name       string
}

// ServerConfigEvent carries updated server information.
type ServerConfigEvent struct{ Info ServerInfo }

// PermissionEvent carries our effective permissions in a channel (ACL bit mask).
// Flush means cached permissions for all channels are stale.
type PermissionEvent struct {
	ChannelID   uint32
	Permissions uint32
	Flush       bool
}

// UserStatsEvent is the answer to RequestUserStats.
type UserStatsEvent struct{ Stats *pb.UserStats }

// ChannelPasswordEvent answers QueryChannelPassword: the password defined by the
// channel's ACL ("" if none).
type ChannelPasswordEvent struct {
	ChannelID uint32
	Password  string
}

// AudioEvent is a received voice packet.
type AudioEvent struct{ Packet AudioPacket }

func (ChannelEvent) isEvent()         {}
func (ChannelRemoveEvent) isEvent()   {}
func (UserEvent) isEvent()            {}
func (UserRemoveEvent) isEvent()      {}
func (TextEvent) isEvent()            {}
func (DeniedEvent) isEvent()          {}
func (ServerConfigEvent) isEvent()    {}
func (AudioEvent) isEvent()           {}
func (PermissionEvent) isEvent()      {}
func (UserStatsEvent) isEvent()       {}
func (ChannelPasswordEvent) isEvent() {}
