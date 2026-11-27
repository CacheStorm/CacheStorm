package store

import (
	"strconv"
	"strings"
)

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
	// NotifyOverwritten and NotifyTypeChanged announce a key replaced
	// wholesale, as Redis' setKey, RENAME, COPY REPLACE and RESTORE REPLACE do.
	NotifyOverwritten = 1 << 15
	NotifyTypeChanged = 1 << 16
	// The subkey channels of Redis 8.10: hash events also name the fields.
	NotifySubkeyspace      = 1 << 19
	NotifySubkeyevent      = 1 << 20
	NotifySubkeyspaceItem  = 1 << 21
	NotifySubkeyspaceEvent = 1 << 22
	NotifyArray            = 1 << 23

	notifySubkeyChannels = NotifySubkeyspace | NotifySubkeyevent | NotifySubkeyspaceItem | NotifySubkeyspaceEvent

	NotifyAll = NotifyGeneric | NotifyString | NotifyList | NotifySet | NotifyHash | NotifyZSet | NotifyExpired | NotifyEvicted | NotifyStream | NotifyModule | NotifyArray
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
		case 'a':
			flags |= NotifyArray
		case 'n':
			flags |= NotifyNew
		case 'o':
			flags |= NotifyOverwritten
		case 'c':
			flags |= NotifyTypeChanged
		case 'S':
			flags |= NotifySubkeyspace
		case 'T':
			flags |= NotifySubkeyevent
		case 'I':
			flags |= NotifySubkeyspaceItem
		case 'V':
			flags |= NotifySubkeyspaceEvent
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
			{NotifyStream, 't'}, {NotifyModule, 'd'}, {NotifyArray, 'a'}, {NotifyNew, 'n'},
			{NotifyOverwritten, 'o'}, {NotifyTypeChanged, 'c'},
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
	for _, f := range []struct {
		flag int
		c    byte
	}{
		{NotifyKeyMiss, 'm'}, {NotifySubkeyspace, 'S'}, {NotifySubkeyevent, 'T'},
		{NotifySubkeyspaceItem, 'I'}, {NotifySubkeyspaceEvent, 'V'},
	} {
		if flags&f.flag != 0 {
			b = append(b, f.c)
		}
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

// NotifyingSubkeys reports whether events of class reach a subkey channel,
// so callers collect the subkeys only when someone may receive them.
func (s *Store) NotifyingSubkeys(class int) bool {
	flags := int(s.pubsub.notifyFlags.Load())
	return flags&class != 0 && flags&notifySubkeyChannels != 0
}

// NotifySubkeys publishes event for key as Notify does and, when subkeys
// are given, on the subkey channels of Redis' notifyKeyspaceEventWithSubkeys:
//
//	__subkeyspace@<db>__:<key>              <event>|<subkeys>
//	__subkeyevent@<db>__:<event>            <len>:<key>|<subkeys>
//	__subkeyspaceitem@<db>__:<key>\n<subkey> <event>   (one per subkey)
//	__subkeyspaceevent@<db>__:<event>|<key> <subkeys>
//
// where <subkeys> is <len>:<subkey>[,<len>:<subkey>...].
func (s *Store) NotifySubkeys(class int, event, key string, subkeys []string) {
	flags := int(s.pubsub.notifyFlags.Load())
	if flags&class == 0 {
		return
	}
	s.publishNotification(flags, event, key)
	if len(subkeys) == 0 || flags&notifySubkeyChannels == 0 {
		return
	}
	db := strconv.AppendInt(nil, int64(s.dbIndex.Load()), 10)
	channel := func(prefix string, parts ...string) string {
		ch := append([]byte(prefix), db...)
		ch = append(ch, "__:"...)
		for _, p := range parts {
			ch = append(ch, p...)
		}
		return string(ch)
	}
	list := func(dst []byte) []byte {
		for i, sk := range subkeys {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = strconv.AppendInt(dst, int64(len(sk)), 10)
			dst = append(dst, ':')
			dst = append(dst, sk...)
		}
		return dst
	}
	piped := strings.IndexByte(event, '|') >= 0
	if flags&NotifySubkeyspace != 0 && !piped {
		payload := append([]byte(event), '|')
		s.pubsub.Publish(channel("__subkeyspace@", key), list(payload))
	}
	if flags&NotifySubkeyevent != 0 {
		payload := strconv.AppendInt(nil, int64(len(key)), 10)
		payload = append(payload, ':')
		payload = append(payload, key...)
		payload = append(payload, '|')
		s.pubsub.Publish(channel("__subkeyevent@", event), list(payload))
	}
	if flags&NotifySubkeyspaceItem != 0 && strings.IndexByte(key, '\n') < 0 {
		for _, sk := range subkeys {
			s.pubsub.Publish(channel("__subkeyspaceitem@", key, "\n", sk), []byte(event))
		}
	}
	if flags&NotifySubkeyspaceEvent != 0 && !piped {
		s.pubsub.Publish(channel("__subkeyspaceevent@", event, "|", key), list(nil))
	}
}

// NotifyOverwrite announces that a write replaced a live key holding a
// value of type old with one of type now, as Redis' setKey does.
func (s *Store) NotifyOverwrite(key string, old, now DataType) {
	if old == 0 {
		return
	}
	s.Notify(NotifyOverwritten, "overwritten", key)
	if old != now {
		s.Notify(NotifyTypeChanged, "type_changed", key)
	}
}

// notifyNew announces that key was added to the keyspace (the "new" event
// of Redis' dbAdd). Loading a dataset announces nothing.
func (s *Store) notifyNew(key string) {
	if s.Notifying(NotifyNew) && !s.clock.loading.Load() {
		s.Notify(NotifyNew, "new", key)
	}
}

// DBIndex returns the index of the database this store currently is.
func (s *Store) DBIndex() int {
	return int(s.dbIndex.Load())
}
