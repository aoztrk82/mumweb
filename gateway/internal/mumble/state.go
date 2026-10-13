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
	// HasDescriptionHash: a long description exists but must be fetched with RequestChannelDescription.
	HasDescriptionHash bool
	Temporary          bool
	MaxUsers           uint32
	Links              []uint32
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
	HasCommentHash  bool     // comment exists but must be fetched with RequestComment
	HasTexture      bool     // user has an avatar (texture)
	Hash            string   // certificate hash
	Listening       []uint32 // channels this user listens to (channel listeners)
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

// applyChannel merges a (possibly partial) ChannelState and returns a copy of the
// result plus copies of other channels whose links changed: links are symmetric
// (Mumble's Channel::link), but the server only announces them on one side.
func (s *State) applyChannel(m *pb.ChannelState) (Channel, []Channel, bool) {
	if m.ChannelId == nil {
		return Channel{}, nil, false
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
		c.HasDescriptionHash = false
	}
	if m.DescriptionHash != nil {
		c.HasDescriptionHash = len(m.DescriptionHash) > 0
		if !c.HasDescriptionHash {
			c.Description = ""
		}
	}
	if m.Temporary != nil {
		c.Temporary = m.GetTemporary()
	}
	if m.MaxUsers != nil {
		c.MaxUsers = m.GetMaxUsers()
	}
	before := slices.Clone(c.Links)
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

	// Mirror link changes on the other side.
	var others []Channel
	for _, l := range c.Links {
		if o := s.channels[l]; o != nil && !slices.Contains(before, l) && !slices.Contains(o.Links, id) {
			o.Links = append(o.Links, id)
			others = append(others, o.clone())
		}
	}
	for _, l := range before {
		if o := s.channels[l]; o != nil && !slices.Contains(c.Links, l) && slices.Contains(o.Links, id) {
			o.Links = slices.DeleteFunc(o.Links, func(x uint32) bool { return x == id })
			others = append(others, o.clone())
		}
	}
	return c.clone(), others, true
}

func (s *State) removeChannel(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.channels, id)
	for _, c := range s.channels {
		c.Links = slices.DeleteFunc(c.Links, func(x uint32) bool { return x == id })
	}
	for _, u := range s.users {
		u.Listening = slices.DeleteFunc(u.Listening, func(x uint32) bool { return x == id })
	}
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
		u.HasCommentHash = false
	}
	if m.CommentHash != nil {
		u.HasCommentHash = len(m.CommentHash) > 0
		if !u.HasCommentHash {
			u.Comment = ""
		}
	}
	if m.Texture != nil {
		u.HasTexture = len(m.Texture) > 0
	}
	if m.TextureHash != nil {
		u.HasTexture = len(m.TextureHash) > 0
	}
	if m.Hash != nil {
		u.Hash = m.GetHash()
	}
	for _, id := range m.ListeningChannelAdd {
		if !slices.Contains(u.Listening, id) {
			u.Listening = append(u.Listening, id)
		}
	}
	for _, id := range m.ListeningChannelRemove {
		u.Listening = slices.DeleteFunc(u.Listening, func(x uint32) bool { return x == id })
	}
	return u.clone(), true
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
		out = append(out, u.clone())
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
	return u.clone(), true
}

func (u *User) clone() User {
	out := *u
	out.Listening = slices.Clone(u.Listening)
	return out
}

func (c *Channel) clone() Channel {
	out := *c
	out.Links = slices.Clone(c.Links)
	return out
}
