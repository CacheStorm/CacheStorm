package command

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/internal/store"
)

// parseRedisInt parses b the way Redis' string2ll does: an optional '-'
// followed by digits with no leading zero ("0" itself is allowed), and nothing
// else. strconv.ParseInt also accepts "+1", "01" and "-0", which Redis
// rejects.
func parseRedisInt(b []byte) (int64, bool) {
	n := len(b)
	if n == 0 || n > 20 {
		return 0, false
	}
	if n == 1 && b[0] == '0' {
		return 0, true
	}
	i := 0
	if b[0] == '-' {
		i = 1
		if n == 1 {
			return 0, false
		}
	}
	if b[i] < '1' || b[i] > '9' {
		return 0, false
	}
	for j := i + 1; j < n; j++ {
		if b[j] < '0' || b[j] > '9' {
			return 0, false
		}
	}
	v, err := strconv.ParseInt(string(b), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// scanHash orders the keyspace for SCAN; the top bit is dropped so that the
// cursor (hash+1) always fits in an unsigned 64-bit integer.

// redisTypeName is the name TYPE reports for v.
func redisTypeName(v store.Value) string {
	return v.Type().String()
}

func errInvalidExpireIn(cmd string) error {
	return errors.New("ERR invalid expire time in '" + cmd + "' command")
}

// stringExpireMs validates and converts the expire argument of SET, SETEX,
// PSETEX and GETEX into an absolute Unix time in milliseconds, following
// Redis' getExpireMillisecondsOrReply.
func stringExpireMs(arg []byte, seconds, relative bool, cmd string) (int64, error) {
	v, ok := parseRedisInt(arg)
	if !ok {
		return 0, ErrNotInteger
	}
	if v <= 0 || (seconds && v > math.MaxInt64/1000) {
		return 0, errInvalidExpireIn(cmd)
	}
	if seconds {
		v *= 1000
	}
	if relative {
		now := time.Now().UnixMilli()
		if v > math.MaxInt64-now {
			return 0, errInvalidExpireIn(cmd)
		}
		v += now
	}
	return v, nil
}

const (
	strOptNX = 1 << iota
	strOptXX
	strOptGET
	strOptKEEPTTL
	strOptPERSIST
	strOptEX
	strOptPX
	strOptEXAT
	strOptPXAT

	strOptExpire = strOptEX | strOptPX | strOptEXAT | strOptPXAT
)

// parseStringOptions parses the options of SET (forSet) or GETEX with the
// rules of Redis' parseExtendedStringArgumentsOrReply: options are
// case-insensitive, an expire option conflicts with the other expire options,
// KEEPTTL and PERSIST, and any conflict or unknown option is a syntax error.
func parseStringOptions(args [][]byte, forSet bool) (int, []byte, error) {
	flags := 0
	var expire []byte
	for j := 0; j < len(args); j++ {
		opt := strings.ToUpper(string(args[j]))
		hasNext := j+1 < len(args)
		expireFlag := 0
		switch opt {
		case "NX":
			if forSet && flags&strOptXX == 0 {
				flags |= strOptNX
				continue
			}
		case "XX":
			if forSet && flags&strOptNX == 0 {
				flags |= strOptXX
				continue
			}
		case "GET":
			if forSet {
				flags |= strOptGET
				continue
			}
		case "KEEPTTL":
			if forSet && flags&(strOptPERSIST|strOptExpire) == 0 {
				flags |= strOptKEEPTTL
				continue
			}
		case "PERSIST":
			if !forSet && flags&(strOptKEEPTTL|strOptExpire) == 0 {
				flags |= strOptPERSIST
				continue
			}
		case "EX":
			expireFlag = strOptEX
		case "PX":
			expireFlag = strOptPX
		case "EXAT":
			expireFlag = strOptEXAT
		case "PXAT":
			expireFlag = strOptPXAT
		}
		if expireFlag != 0 && hasNext && flags&(strOptKEEPTTL|strOptPERSIST|(strOptExpire&^expireFlag)) == 0 {
			flags |= expireFlag
			expire = args[j+1]
			j++
			continue
		}
		return 0, nil, ErrSyntaxError
	}
	return flags, expire, nil
}

// stringOptionsExpireMs converts the parsed expire argument into an absolute
// millisecond instant.
func stringOptionsExpireMs(flags int, expire []byte, cmd string) (int64, error) {
	return stringExpireMs(expire, flags&(strOptEX|strOptEXAT) != 0, flags&(strOptEX|strOptPX) != 0, cmd)
}

// parseInt64 parses a decimal integer, 0 when s is not one.
func parseInt64(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
