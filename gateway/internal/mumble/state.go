package mumble

import (
	"cmp"
	"slices"
	"sync"

	pb "github.com/aoztrk82/mumweb/gateway/proto/mumbleproto"
)

// Channel is the merged state of a Mumble channel.
type Channel struct {
	ID          uint32
	Parent      uint32 // meaningless for the root channel (ID 0)
	Name        string
	Position    int32
	Description string
	Temporary   bool
	MaxUsers    uint32
	Links       []uint32
}

// User is the merged state of a connected user.
type User struct {
	Session         uint32
	Name            string
	UserID          uint32
	Registered      bool // UserID is only meaningful when Registered
	ChannelID       uint32
	Mute            bool
	Deaf            bool
	Suppress        bool
	SelfMute        bool
	SelfDeaf        bool
	PrioritySpeaker bool
	Recording       bool
	Comment         string
	Hash            string // certificate hash
}

// State holds the channel and user maps. It is safe for concurrent use.
type State struct {
	mu       sync.RWMutex
	channels map[uint32]*Channel
	users    map[uint32]*User
}

func newState() *State {
	return &State{channels: map[uint32]*Channel{}, users: map[uint32]*User{}}
}

// applyChannel merges a (possibly partial) ChannelState and returns a copy of the result.
func (s *State) applyChannel(m *pb.ChannelState) (Channel, bool) {
	if m.ChannelId == nil {
		return Channel{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := m.GetChannelId()
	c, ok := s.channels[id]
	if !ok {
		c = &Channel{ID: id}
		s.channels[id] = c
	}
	if m.Parent != nil {
		c.Parent = m.GetParent()
	}
	if m.Name != nil {
		c.Name = m.GetName()
	}
	if m.Position != nil {
		c.Position = m.GetPosition()
	}
	if m.Description != nil {
		c.Description = m.GetDescription()
	}
	if m.Temporary != nil {
		c.Temporary = m.GetTemporary()
	}
	if m.MaxUsers != nil {
		c.MaxUsers = m.GetMaxUsers()
	}
	if m.Links != nil {
		c.Links = slices.Clone(m.Links)
	}
	for _, l := range m.LinksAdd {
		if !slices.Contains(c.Links, l) {
			c.Links = append(c.Links, l)
		}
	}
	for _, l := range m.LinksRemove {
		c.Links = slices.DeleteFunc(c.Links, func(x uint32) bool { return x == l })
	}
	return c.clone(), true
}

func (s *State) removeChannel(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.channels, id)
}

// applyUser merges a (possibly partial) UserState and returns a copy of the result.
func (s *State) applyUser(m *pb.UserState) (User, bool) {
	if m.Session == nil {
		return User{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := m.GetSession()
	u, ok := s.users[sess]
	if !ok {
		// A new user without channel_id is in the root channel (ID 0).
		u = &User{Session: sess}
		s.users[sess] = u
	}
	if m.Name != nil {
		u.Name = m.GetName()
	}
	if m.UserId != nil {
		u.UserID = m.GetUserId()
		u.Registered = true
	}
	if m.ChannelId != nil {
		u.ChannelID = m.GetChannelId()
	}
	if m.Mute != nil {
		u.Mute = m.GetMute()
	}
	if m.Deaf != nil {
		u.Deaf = m.GetDeaf()
	}
	if m.Suppress != nil {
		u.Suppress = m.GetSuppress()
	}
	if m.SelfMute != nil {
		u.SelfMute = m.GetSelfMute()
	}
	if m.SelfDeaf != nil {
		u.SelfDeaf = m.GetSelfDeaf()
	}
	if m.PrioritySpeaker != nil {
		u.PrioritySpeaker = m.GetPrioritySpeaker()
	}
	if m.Recording != nil {
		u.Recording = m.GetRecording()
	}
	if m.Comment != nil {
		u.Comment = m.GetComment()
	}
	if m.Hash != nil {
		u.Hash = m.GetHash()
	}
	return *u, true
}

func (s *State) removeUser(session uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.users, session)
}

// Channels returns a copy of all channels sorted by ID.
func (s *State) Channels() []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Channel, 0, len(s.channels))
	for _, c := range s.channels {
		out = append(out, c.clone())
	}
	slices.SortFunc(out, func(a, b Channel) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// Users returns a copy of all users sorted by session.
func (s *State) Users() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, *u)
	}
	slices.SortFunc(out, func(a, b User) int { return cmp.Compare(a.Session, b.Session) })
	return out
}

// User returns a copy of one user.
func (s *State) User(session uint32) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[session]
	if !ok {
		return User{}, false
	}
	return *u, true
}

func (c *Channel) clone() Channel {
	out := *c
	out.Links = slices.Clone(c.Links)
	return out
}
