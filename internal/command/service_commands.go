package command

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"

	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

// A SERVICE registry is a hash key named after the service, marked by the
// field __service, with one field per instance id holding the instance as a
// JSON object. Being a key, a registry is per database, persisted and
// replicated; registrations, heartbeats and changes are recorded as HSET,
// deregistrations as HDEL. An instance is healthy while its last heartbeat
// is under serviceTTLMS old.

const (
	serviceMarker = "__service"
	serviceTTLMS  = 30000
)

var (
	errServiceNotFound = errors.New("ERR service not found")
	errServicePort     = errors.New("ERR port must be an integer between 0 and 65535")
	errServiceWeight   = errors.New("ERR weight must be a non-negative integer")
	errServiceID       = errors.New("ERR instance id __service is reserved")
)

type serviceInstance struct {
	Address    string   `json:"address"`
	Port       int64    `json:"port"`
	Weight     int64    `json:"weight"`
	Tags       []string `json:"tags"`
	LastSeen   int64    `json:"last_seen"`
	Registered int64    `json:"registered"`
}

// readServices returns the instances of the registry at name by id, nil when
// it does not exist.
func readServices(ctx *Context, name string) (map[string]*serviceInstance, error) {
	h, err := readMarkedHash(ctx, name, serviceMarker)
	if err != nil || h == nil {
		return nil, err
	}
	fields := markedFields(h, serviceMarker)
	out := make(map[string]*serviceInstance, len(fields))
	for id, raw := range fields {
		inst := &serviceInstance{}
		if json.Unmarshal(raw, inst) != nil {
			return nil, store.ErrWrongType
		}
		out[id] = inst
	}
	return out, nil
}

func loadServiceInstance(ctx *Context, name, id string) (*serviceInstance, error) {
	instances, err := readServices(ctx, name)
	if err != nil {
		return nil, err
	}
	inst, ok := instances[id]
	if !ok {
		return nil, errServiceNotFound
	}
	return inst, nil
}

func saveServiceInstance(ctx *Context, name, id string, inst *serviceInstance) error {
	raw, err := json.Marshal(inst)
	if err != nil {
		return err
	}
	return setMarkedFields(ctx, name, serviceMarker, id, string(raw))
}

// SERVICE.REGISTER name id address port [weight]
func cmdSERVICEREGISTER(ctx *Context) error {
	if ctx.ArgCount() < 4 || ctx.ArgCount() > 5 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	if id == serviceMarker {
		return ctx.WriteError(errServiceID)
	}
	port, err := strconv.ParseInt(ctx.ArgString(3), 10, 64)
	if err != nil || port < 0 || port > 65535 {
		return ctx.WriteError(errServicePort)
	}
	weight := int64(100)
	if ctx.ArgCount() == 5 {
		if weight, err = strconv.ParseInt(ctx.ArgString(4), 10, 64); err != nil || weight < 0 {
			return ctx.WriteError(errServiceWeight)
		}
	}
	if _, err := readServices(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	now := ctx.nowMS()
	inst := &serviceInstance{Address: ctx.ArgString(2), Port: port, Weight: weight, Tags: []string{}, LastSeen: now, Registered: now}
	if err := saveServiceInstance(ctx, name, id, inst); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSERVICEDEREGISTER(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	if _, err := readServices(ctx, name); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteInteger(int64(deleteMarkedFields(ctx, name, serviceMarker, id)))
}

// healthyServices returns the ids of the instances heard from recently,
// sorted.
func healthyServices(ctx *Context, instances map[string]*serviceInstance) []string {
	now := ctx.nowMS()
	var ids []string
	for id, inst := range instances {
		if now-inst.LastSeen < serviceTTLMS {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func cmdSERVICEDISCOVER(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	instances, err := readServices(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	ids := healthyServices(ctx, instances)
	results := make([]*resp.Value, 0, len(ids))
	for _, id := range ids {
		inst := instances[id]
		tags := make([]*resp.Value, len(inst.Tags))
		for i, t := range inst.Tags {
			tags[i] = resp.BulkString(t)
		}
		results = append(results, resp.ArrayValue([]*resp.Value{
			resp.BulkString("id"), resp.BulkString(id),
			resp.BulkString("address"), resp.BulkString(inst.Address),
			resp.BulkString("port"), resp.IntegerValue(inst.Port),
			resp.BulkString("weight"), resp.IntegerValue(inst.Weight),
			resp.BulkString("tags"), resp.ArrayValue(tags),
		}))
	}
	return ctx.WriteArray(results)
}

func cmdSERVICEHEALTHY(ctx *Context) error {
	if ctx.ArgCount() != 1 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	instances, err := readServices(ctx, ctx.ArgString(0))
	if err != nil {
		return ctx.WriteError(err)
	}
	ids := healthyServices(ctx, instances)
	out := make([]*resp.Value, len(ids))
	for i, id := range ids {
		out[i] = resp.BulkString(id)
	}
	return ctx.WriteArray(out)
}

// SERVICE.LIST replies the service names of the selected database, sorted.
func cmdSERVICELIST(ctx *Context) error {
	if ctx.ArgCount() != 0 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	names := markedKeys(ctx, serviceMarker)
	out := make([]*resp.Value, len(names))
	for i, n := range names {
		out[i] = resp.BulkString(n)
	}
	return ctx.WriteArray(out)
}

// updateService changes one instance with change and saves it.
func updateService(ctx *Context, change func(*serviceInstance)) error {
	name, id := ctx.ArgString(0), ctx.ArgString(1)
	inst, err := loadServiceInstance(ctx, name, id)
	if err != nil {
		return ctx.WriteError(err)
	}
	change(inst)
	if err := saveServiceInstance(ctx, name, id, inst); err != nil {
		return ctx.WriteError(err)
	}
	return ctx.WriteOK()
}

func cmdSERVICEHEARTBEAT(ctx *Context) error {
	if ctx.ArgCount() != 2 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	now := ctx.nowMS()
	return updateService(ctx, func(inst *serviceInstance) { inst.LastSeen = now })
}

func cmdSERVICEWEIGHT(ctx *Context) error {
	if ctx.ArgCount() != 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	weight, err := strconv.ParseInt(ctx.ArgString(2), 10, 64)
	if err != nil || weight < 0 {
		return ctx.WriteError(errServiceWeight)
	}
	return updateService(ctx, func(inst *serviceInstance) { inst.Weight = weight })
}

// SERVICE.TAGS name id tag [tag ...] replaces the instance's tags.
func cmdSERVICETAGS(ctx *Context) error {
	if ctx.ArgCount() < 3 {
		return ctx.WriteError(ErrWrongArgCount)
	}
	tags := make([]string, 0, ctx.ArgCount()-2)
	for i := 2; i < ctx.ArgCount(); i++ {
		tags = append(tags, ctx.ArgString(i))
	}
	return updateService(ctx, func(inst *serviceInstance) { inst.Tags = tags })
}
