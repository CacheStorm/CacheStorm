package persistence

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// RedisAOF is a Redis 7 multi-part AOF, as its manifest names it: the base
// file (an RDB or a command file; empty when there is none) and the
// incremental command files to apply after it, in order, as paths.
type RedisAOF struct {
	Base string
	Incr []string
}

// ReadRedisAOFManifest reads the manifest Redis 7 keeps for the AOF
// filename in dir (appendonlydir and appendonly.aof by default). It returns
// an error wrapping os.ErrNotExist when there is no manifest.
func ReadRedisAOFManifest(dir, filename string) (*RedisAOF, error) {
	path := filepath.Join(dir, filename+".manifest")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	aof := &RedisAOF{}
	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || text[0] == '#' {
			continue
		}
		fields, err := splitRedisArgs(text)
		if err != nil || len(fields)%2 != 0 {
			return nil, fmt.Errorf("AOF manifest %s line %d: invalid format", path, line)
		}
		kv := map[string]string{}
		for i := 0; i < len(fields); i += 2 {
			kv[fields[i]] = fields[i+1]
		}
		name := kv["file"]
		if name == "" || strings.ContainsAny(name, `/\`) {
			return nil, fmt.Errorf("AOF manifest %s line %d: invalid file name", path, line)
		}
		if _, err := strconv.ParseInt(kv["seq"], 10, 64); err != nil {
			return nil, fmt.Errorf("AOF manifest %s line %d: invalid seq", path, line)
		}
		switch kv["type"] {
		case "b":
			if aof.Base != "" {
				return nil, fmt.Errorf("AOF manifest %s: more than one base file", path)
			}
			aof.Base = filepath.Join(dir, name)
		case "i":
			aof.Incr = append(aof.Incr, filepath.Join(dir, name))
		case "h":
			// A history file is left for deletion and not loaded.
		default:
			return nil, fmt.Errorf("AOF manifest %s line %d: unknown file type %q", path, line, kv["type"])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if aof.Base == "" && len(aof.Incr) == 0 {
		return nil, fmt.Errorf("AOF manifest %s names no files", path)
	}
	return aof, nil
}

// splitRedisArgs splits a line as Redis' sdssplitargs does: words separated
// by spaces, a word in double quotes taking \n, \r, \t, \b, \a, \xHH and
// backslash escapes, one in single quotes only \'.
func splitRedisArgs(line string) ([]string, error) {
	var out []string
	for i := 0; ; {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i == len(line) {
			return out, nil
		}
		var word strings.Builder
		switch line[i] {
		case '"':
			i++
			for {
				if i >= len(line) {
					return nil, errors.New("unbalanced quotes")
				}
				c := line[i]
				if c == '"' {
					i++
					break
				}
				if c == '\\' && i+1 < len(line) {
					i++
					switch e := line[i]; e {
					case 'n':
						c = '\n'
					case 'r':
						c = '\r'
					case 't':
						c = '\t'
					case 'b':
						c = '\b'
					case 'a':
						c = '\a'
					case 'x':
						if i+2 < len(line) {
							if v, err := strconv.ParseUint(line[i+1:i+3], 16, 8); err == nil {
								c = byte(v)
								i += 2
								break
							}
						}
						c = 'x'
					default:
						c = e
					}
				}
				word.WriteByte(c)
				i++
			}
		case '\'':
			i++
			for {
				if i >= len(line) {
					return nil, errors.New("unbalanced quotes")
				}
				c := line[i]
				if c == '\'' {
					i++
					break
				}
				if c == '\\' && i+1 < len(line) && line[i+1] == '\'' {
					i++
					c = '\''
				}
				word.WriteByte(c)
				i++
			}
		default:
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				word.WriteByte(line[i])
				i++
			}
			out = append(out, word.String())
			continue
		}
		// A closing quote must be followed by a space or the end.
		if i < len(line) && line[i] != ' ' && line[i] != '\t' {
			return nil, errors.New("closing quote must be followed by a space")
		}
		out = append(out, word.String())
	}
}

// IsRDBFile reports whether the file at path starts with an RDB header.
func IsRDBFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var magic [5]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	return string(magic[:]) == "REDIS", nil
}
