package command

import (
	"errors"
	"strconv"
	"strings"

	"github.com/cachestorm/cachestorm/internal/store"
)

func cmdRESTORELegacy(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}

	key := ctx.ArgString(0)
	ttl, err := strconv.ParseInt(ctx.ArgString(1), 10, 64)
	if err != nil {
		return ctx.WriteError(ErrNotInteger)
	}

	serialized := ctx.ArgString(2)

	if len(serialized) < 13 || !strings.HasPrefix(serialized, "CACHSTORM001") {
		return ctx.WriteError(errors.New("ERR DUMP payload version or checksum are wrong"))
	}

	replace := false
	for i := 3; i < ctx.ArgCount(); i++ {
		if strings.ToUpper(ctx.ArgString(i)) == "REPLACE" {
			replace = true
		}
	}

	if !replace {
		if _, exists := ctx.Store.Get(key); exists {
			return ctx.WriteError(errors.New("BUSYKEY Target key name already exists"))
		}
	}

	data := serialized[12:]
	typeByte := data[0]
	data = data[1:]

	colonIdx := strings.Index(data, ":")
	if colonIdx == -1 {
		return ctx.WriteError(errors.New("ERR invalid DUMP format"))
	}

	_ = data[:colonIdx]
	data = data[colonIdx+1:]

	var value store.Value
	switch store.DataType(typeByte) {
	case store.DataTypeString:
		value = &store.StringValue{Data: []byte(data)}
	case store.DataTypeHash:
		hv := &store.HashValue{Fields: make(map[string][]byte)}
		if len(data) > 0 {
			pairs := strings.Split(data, "&")
			for _, pair := range pairs {
				if pair == "" {
					continue
				}
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 2 {
					hv.SetField(kv[0], []byte(kv[1]))
				}
			}
		}
		value = hv
	case store.DataTypeList:
		lv := &store.ListValue{Elements: make([][]byte, 0)}
		if len(data) > 0 {
			items := strings.Split(data, ",")
			for _, item := range items {
				if item != "" {
					lv.Elements = append(lv.Elements, []byte(item))
				}
			}
		}
		value = lv
	case store.DataTypeSet:
		sv := &store.SetValue{Members: make(map[string]struct{})}
		if len(data) > 0 {
			items := strings.Split(data, ",")
			for _, item := range items {
				if item != "" {
					sv.AddMember(item)
				}
			}
		}
		value = sv
	default:
		return ctx.WriteError(errors.New("ERR unsupported type for RESTORE"))
	}

	if err := ctx.Store.Set(key, value, store.SetOptions{ExpireAtMS: max(ttl, 0)}); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}
