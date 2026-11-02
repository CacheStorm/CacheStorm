package search

import (
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

// FT.CONFIG's options, with RediSearch's names, help texts, defaults and
// validation (config_table.go). Like RediSearch's, they belong to the
// process. The ones that change behaviour here are read through the
// accessors below.

type configKind int

const (
	configImmutable configKind = iota
	configGe0
	configGe1
	configUnlimited
	configBool
	configFlag
	configStemlen
	configDialect
	configOntimeout
	configScorer
	configRangeparents
)

type configOption struct {
	value *string
	name  string
	help  string
	kind  configKind
}

func ptr(s string) *string { return &s }

var (
	configMu     sync.RWMutex
	configValues map[string]*string
)

func configValue(name string) *string {
	configMu.RLock()
	defer configMu.RUnlock()
	if configValues == nil {
		return configDefault(name)
	}
	return configValues[name]
}

func configDefault(name string) *string {
	for _, o := range configOptions {
		if o.name == name {
			return o.value
		}
	}
	return nil
}

func configInt(name string, fallback int64) int64 {
	if v := configValue(name); v != nil {
		if n, err := strconv.ParseInt(*v, 10, 64); err == nil {
			return n
		}
	}
	return fallback
}

// ConfigOption is one option as FT.CONFIG GET and HELP show it.
type ConfigOption struct {
	Value *string
	Name  string
	Help  string
}

// ConfigGet returns the option name names (any case), or all for "*".
func ConfigGet(name string) []ConfigOption {
	configMu.RLock()
	defer configMu.RUnlock()
	var out []ConfigOption
	for _, o := range configOptions {
		if name == "*" || strings.EqualFold(name, o.name) {
			v := o.value
			if configValues != nil {
				v = configValues[o.name]
			}
			out = append(out, ConfigOption{Name: o.name, Help: o.help, Value: v})
		}
	}
	return out
}

//nolint:staticcheck // RediSearch's reply texts
var (
	errConfigInvalid   = errors.New("Invalid option")
	errConfigImmutable = errors.New("Not modifiable at runtime")
	errConfigType      = errors.New("Could not convert argument to expected type")
	errConfigBounds    = errors.New("Value is outside acceptable bounds")
	errConfigNoArg     = errors.New("Expected an argument, but none provided")
)

// ConfigSet sets an option from args; excess reports arguments it did not
// use, which RediSearch answers with EXCESSARGS after setting the option.
func ConfigSet(name string, args []string) (excess bool, err error) {
	var opt *configOption
	for i := range configOptions {
		if strings.EqualFold(name, configOptions[i].name) {
			opt = &configOptions[i]
		}
	}
	if opt == nil {
		return false, errConfigInvalid
	}
	if opt.kind == configImmutable {
		return false, errConfigImmutable
	}
	if opt.kind == configFlag {
		setConfig(opt.name, "true")
		return len(args) > 0, nil
	}
	if len(args) == 0 {
		return false, errConfigNoArg
	}
	v, err := opt.parse(args[0])
	if err != nil {
		return false, err
	}
	setConfig(opt.name, v)
	switch opt.name {
	case "MAXEXPANSIONS":
		setConfig("MAXPREFIXEXPANSIONS", v)
	case "MAXPREFIXEXPANSIONS":
		setConfig("MAXEXPANSIONS", v)
	}
	return len(args) > 1, nil
}

func setConfig(name, v string) {
	configMu.Lock()
	defer configMu.Unlock()
	if configValues == nil {
		configValues = make(map[string]*string, len(configOptions))
		for _, o := range configOptions {
			configValues[o.name] = o.value
		}
	}
	configValues[name] = &v
}

func (o *configOption) parse(arg string) (string, error) {
	switch o.kind {
	case configBool:
		switch strings.ToLower(arg) {
		case "true", "false":
			return strings.ToLower(arg), nil
		}
		return "", errConfigType
	case configOntimeout:
		switch strings.ToLower(arg) {
		case "return", "fail":
			return strings.ToLower(arg), nil
		}
		return "", errors.New("Invalid ON_TIMEOUT value") //nolint:staticcheck // RediSearch's reply text
	case configScorer:
		if v := configValue("ENABLE_UNSTABLE_FEATURES"); v == nil || *v != "true" {
			return "", errors.New("`search-default-scorer` is unavailable when `search-enable-unstable-features` is off. Enable it with `CONFIG SET search-enable-unstable-features true`")
		}
		return strings.ToUpper(arg), nil
	case configDialect:
		n, err := strconv.ParseUint(arg, 10, 32)
		if err != nil || n < 1 || n > 4 {
			return "", errors.New("Default dialect version cannot be higher than 4") //nolint:staticcheck // RediSearch's reply text
		}
		return arg, nil
	case configRangeparents:
		n, err := strconv.ParseUint(arg, 10, 64)
		if err != nil || n > 2 {
			return "", errors.New("Max depth for range cannot be higher than max depth for balance") //nolint:staticcheck // RediSearch's reply text
		}
		return strconv.FormatUint(n, 10), nil
	}
	n, err := strconv.ParseInt(arg, 10, 64)
	if err != nil {
		return "", errConfigType
	}
	switch o.kind {
	case configUnlimited:
		if n < 0 {
			return "unlimited", nil
		}
	case configGe0:
		if n < 0 {
			return "", errConfigBounds
		}
	case configGe1, configStemlen:
		if n < 1 {
			return "", errConfigBounds
		}
		if o.kind == configStemlen && n < 2 {
			return "", errors.New("Minimum stem length cannot be lower than 2") //nolint:staticcheck // RediSearch's reply text
		}
	}
	return strconv.FormatInt(n, 10), nil
}

// DefaultDialect is the dialect of queries that name none.
func DefaultDialect() int { return int(configInt("DEFAULT_DIALECT", 1)) }

func minPrefixLength() int     { return int(configInt("MINPREFIX", 2)) }
func maxPrefixExpansions() int { return int(configInt("MAXPREFIXEXPANSIONS", 200)) }
func minStemLength() int       { return int(configInt("MINSTEMLEN", 4)) }
func unionHeapMin() int        { return int(configInt("UNION_ITERATOR_HEAP", 20)) }

// CursorMaxIdle is the longest MAXIDLE a cursor may have.
func CursorMaxIdle() time.Duration {
	return time.Duration(configInt("CURSOR_MAX_IDLE", 300000)) * time.Millisecond
}

// IndexCursorLimit is how many cursors an index may hold.
func IndexCursorLimit() int { return int(configInt("INDEX_CURSOR_LIMIT", 128)) }

// MaxSearchResults is FT.SEARCH's largest LIMIT, negative for unlimited.
func MaxSearchResults() int64 { return configInt("MAXSEARCHRESULTS", -1) }

// MaxAggregateResults is FT.AGGREGATE's largest LIMIT, negative for none.
func MaxAggregateResults() int64 { return configInt("MAXAGGREGATERESULTS", -1) }
