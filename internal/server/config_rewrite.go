package server

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// rewriteConfig is CONFIG REWRITE: the settings CONFIG SET can change are
// written back into the YAML file the server was started from. The file is
// edited as a YAML node tree, so its comments, order and other settings are
// kept, and it is replaced atomically.
func (s *Server) rewriteConfig() error {
	path := s.cfg.SourceFile
	if path == "" {
		return errors.New("ERR The server is running without a config file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("ERR Rewriting config file: %v", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("ERR Rewriting config file: %v", err)
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return errors.New("ERR Rewriting config file: the top level is not a mapping")
	}
	limit, policy := s.store.MemoryLimit()
	samples := s.cfg.Memory.SampleSize
	if ev := s.store.Evictor(); ev != nil {
		samples = ev.SampleSize()
	}
	idle := time.Duration(s.idleTimeout.Load())
	setYAML(root, []string{"memory", "max_memory"}, strconv.FormatInt(limit, 10))
	setYAML(root, []string{"memory", "eviction_policy"}, policy.String())
	setYAML(root, []string{"memory", "eviction_sample_size"}, strconv.Itoa(samples))
	setYAML(root, []string{"server", "max_connections"}, strconv.FormatInt(s.maxClients.Load(), 10))
	setYAML(root, []string{"server", "read_timeout"}, idle.String())
	// A password supplied by CACHESTORM_REQUIREPASS stays out of the file;
	// only one set with CONFIG SET is written.
	if pass := s.router.RequirePass(); pass != s.cfg.Server.RequirePass {
		setYAML(root, []string{"server", "requirepass"}, pass)
	}

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("ERR Rewriting config file: %v", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("ERR Rewriting config file: %v", err)
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cachestorm-config-*")
	if err != nil {
		return fmt.Errorf("ERR Rewriting config file: %v", err)
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(out.Bytes())
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), mode)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return fmt.Errorf("ERR Rewriting config file: %v", err)
	}
	return nil
}

// setYAML sets the scalar at path in a mapping node, creating the
// intermediate mappings and the key when they are missing. Strings are
// quoted so that values such as "0" stay strings.
func setYAML(node *yaml.Node, path []string, value string) {
	for i, key := range path {
		var child *yaml.Node
		for j := 0; j+1 < len(node.Content); j += 2 {
			if node.Content[j].Value == key {
				child = node.Content[j+1]
				break
			}
		}
		last := i == len(path)-1
		if child == nil {
			child = &yaml.Node{Kind: yaml.MappingNode}
			if last {
				child = &yaml.Node{Kind: yaml.ScalarNode}
			}
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, child)
		}
		if last {
			child.Kind = yaml.ScalarNode
			child.Tag = ""
			child.Value = value
			if _, err := strconv.ParseInt(value, 10, 64); err == nil {
				child.Style = 0
			} else {
				child.Style = yaml.DoubleQuotedStyle
			}
			return
		}
		if child.Kind != yaml.MappingNode {
			child.Kind = yaml.MappingNode
			child.Value = ""
			child.Content = nil
		}
		node = child
	}
}
