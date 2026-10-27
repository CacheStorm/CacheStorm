package store

import "strconv"

// Keyspace notification classes, the NOTIFY_* flags of Redis' server.h.
const (
	NotifyKeyspace = 1 << 0
	NotifyKeyevent = 1 << 1
	NotifyGeneric  = 1 << 2
	NotifyString   = 1 << 3
	NotifyList     = 1 << 4
	NotifySet      = 1 << 5
	NotifyHash     = 1 << 6
	NotifyZSet     = 1 << 7
	NotifyExpired  = 1 << 8
	NotifyEvicted  = 1 << 9
	NotifyStream   = 1 << 10
	NotifyKeyMiss  = 1 << 11
	NotifyLoaded   = 1 << 12
	NotifyModule   = 1 << 13
	NotifyNew      = 1 << 14

	NotifyAll = NotifyGeneric | NotifyString | NotifyList | NotifySet | NotifyHash | NotifyZSet | NotifyExpired | NotifyEvicted | NotifyStream | NotifyModule
)

// ParseNotifyFlags parses a notify-keyspace-events value as Redis'
// keyspaceEventsStringToFlags does; ok is false when a character names no
// event class.
func ParseNotifyFlags(s string) (flags int, ok bool) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case 'A':
			flags |= NotifyAll
		case 'g':
			flags |= NotifyGeneric
		case '$':
			flags |= NotifyString
		case 'l':
			flags |= NotifyList
		case 's':
			flags |= NotifySet
		case 'h':
			flags |= NotifyHash
		case 'z':
			flags |= NotifyZSet
		case 'x':
			flags |= NotifyExpired
		case 'e':
			flags |= NotifyEvicted
		case 'K':
			flags |= NotifyKeyspace
		case 'E':
			flags |= NotifyKeyevent
		case 't':
			flags |= NotifyStream
		case 'm':
			flags |= NotifyKeyMiss
		case 'd':
			flags |= NotifyModule
		case 'n':
			flags |= NotifyNew
		default:
			return 0, false
		}
	}
	return flags, true
}

// FormatNotifyFlags renders flags the way CONFIG GET notify-keyspace-events
// shows them (Redis' keyspaceEventsFlagsToString).
func FormatNotifyFlags(flags int) string {
	var b []byte
	if flags&NotifyAll == NotifyAll {
		b = append(b, 'A')
	} else {
		for _, f := range []struct {
			flag int
			c    byte
		}{
			{NotifyGeneric, 'g'}, {NotifyString, '$'}, {NotifyList, 'l'}, {NotifySet, 's'},
			{NotifyHash, 'h'}, {NotifyZSet, 'z'}, {NotifyExpired, 'x'}, {NotifyEvicted, 'e'},
			{NotifyStream, 't'}, {NotifyModule, 'd'}, {NotifyNew, 'n'},
		} {
			if flags&f.flag != 0 {
				b = append(b, f.c)
			}
		}
	}
	if flags&NotifyKeyspace != 0 {
		b = append(b, 'K')
	}
	if flags&NotifyKeyevent != 0 {
		b = append(b, 'E')
	}
	if flags&NotifyKeyMiss != 0 {
		b = append(b, 'm')
	}
	return string(b)
}

// SetNotifyFlags sets the keyspace notification classes of every database
// sharing this store's Pub/Sub hub.
func (s *Store) SetNotifyFlags(flags int) {
	s.pubsub.notifyFlags.Store(int32(flags))
}

// NotifyFlags returns the keyspace notification classes in effect.
func (s *Store) NotifyFlags() int {
	return int(s.pubsub.notifyFlags.Load())
}

// Notifying reports whether events of class are published.
func (s *Store) Notifying(class int) bool {
	return int(s.pubsub.notifyFlags.Load())&class != 0
}

// Notify publishes the keyspace notification event for key, as Redis'
// notifyKeyspaceEvent: to __keyspace@<db>__:<key> with the event as the
// message, and to __keyevent@<db>__:<event> with the key as the message.
// Delivery is to this server's subscribers only. With the class disabled it
// costs one atomic load.
func (s *Store) Notify(class int, event, key string) {
	flags := int(s.pubsub.notifyFlags.Load())
	if flags&class == 0 {
		return
	}
	s.publishNotification(flags, event, key)
}

func (s *Store) publishNotification(flags int, event, key string) {
	if flags&(NotifyKeyspace|NotifyKeyevent) == 0 {
		return
	}
	db := s.dbIndex.Load()
	if flags&NotifyKeyspace != 0 {
		ch := make([]byte, 0, len("__keyspace@__:")+4+len(key))
		ch = append(ch, "__keyspace@"...)
		ch = strconv.AppendInt(ch, int64(db), 10)
		ch = append(ch, "__:"...)
		ch = append(ch, key...)
		s.pubsub.Publish(string(ch), []byte(event))
	}
	if flags&NotifyKeyevent != 0 {
		ch := make([]byte, 0, len("__keyevent@__:")+4+len(event))
		ch = append(ch, "__keyevent@"...)
		ch = strconv.AppendInt(ch, int64(db), 10)
		ch = append(ch, "__:"...)
		ch = append(ch, event...)
		s.pubsub.Publish(string(ch), []byte(key))
	}
}

// notifyNew announces that key was added to the keyspace (the "new" event
// of Redis' dbAdd). Loading a dataset announces nothing.
func (s *Store) notifyNew(key string) {
	if s.Notifying(NotifyNew) && !s.loading.Load() {
		s.Notify(NotifyNew, "new", key)
	}
}

// DBIndex returns the index of the database this store currently is.
func (s *Store) DBIndex() int {
	return int(s.dbIndex.Load())
}
