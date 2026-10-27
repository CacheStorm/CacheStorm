package persistence

import (
	"os"
	"testing"
)

func TestAOFWriteFailureLatchesHealth(t *testing.T) {
	for _, phase := range []string{"always-flush", "always-sync", "background-flush", "background-sync", "explicit-flush"} {
		t.Run(phase, func(t *testing.T) {
			device := "/dev/full"
			if phase == "always-sync" || phase == "background-sync" {
				device = "/dev/null"
			}
			if _, err := os.Stat(device); err != nil {
				t.Skip("requires Unix test devices")
			}
			policy := AOFNoSync
			if phase == "always-flush" || phase == "always-sync" {
				policy = AOFAlways
			}
			w := NewAOFWriter(AOFConfig{Enabled: true, DataDir: t.TempDir(), Filename: "appendonly.aof", SyncPolicy: policy})
			w.openFile = func(_ string, flags int, mode os.FileMode) (*os.File, error) { return os.OpenFile(device, flags, mode) }
			if err := w.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(w.Stop)
			err := w.Append("SET", [][]byte{[]byte("first"), []byte("v")})
			switch phase {
			case "background-flush", "background-sync":
				if err != nil {
					t.Fatal(err)
				}
				w.syncFile()
			case "explicit-flush":
				if err != nil {
					t.Fatal(err)
				}
				if err := w.Flush(); err == nil {
					t.Fatal("flush succeeded on full device")
				}
			default:
				if err == nil {
					t.Fatal("always append succeeded despite persistence failure")
				}
			}
			if w.Err() == nil {
				t.Fatal("failure was not retained")
			}
			if err := w.Append("SET", [][]byte{[]byte("later"), []byte("v")}); err == nil {
				t.Fatal("append succeeded after persistent failure")
			}
			if err := w.Flush(); err == nil {
				t.Fatal("flush succeeded after persistent failure")
			}
		})
	}
}

func TestAOFShutdownReturnsFlushAndSyncErrors(t *testing.T) {
	for _, device := range []string{"/dev/full", "/dev/null"} {
		t.Run(device, func(t *testing.T) {
			if _, err := os.Stat(device); err != nil {
				t.Skip("requires Unix test devices")
			}
			w := NewAOFWriter(AOFConfig{Enabled: true, DataDir: t.TempDir(), Filename: "appendonly.aof", SyncPolicy: AOFNoSync})
			w.openFile = func(_ string, flags int, mode os.FileMode) (*os.File, error) { return os.OpenFile(device, flags, mode) }
			if err := w.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(w.Stop)
			if err := w.Append("SET", [][]byte{[]byte("buffered"), []byte("v")}); err != nil {
				t.Fatal(err)
			}
			if err := w.StopWithError(); err == nil {
				t.Fatal("shutdown hid persistence failure")
			}
			if w.running.Load() || w.Err() == nil {
				t.Fatal("shutdown failed to stop and retain the failure")
			}
			if err := w.Start(); err == nil {
				t.Fatal("failed writer restarted without recovery")
			}
		})
	}
}
