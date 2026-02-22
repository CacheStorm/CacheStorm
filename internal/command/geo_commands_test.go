package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func TestAllGeoCommands(t *testing.T) {
	s := store.NewStore()
	router := NewRouter()
	RegisterGeoCommands(router)

	tests := []struct {
		name  string
		cmd   string
		args  [][]byte
		setup func()
	}{
		{"GEOADD single", "GEOADD", [][]byte{[]byte("geo1"), []byte("13.361389"), []byte("38.115556"), []byte("Palermo")}, nil},
		{"GEOADD multiple", "GEOADD", [][]byte{[]byte("geo2"), []byte("13.361389"), []byte("38.115556"), []byte("Palermo"), []byte("15.087269"), []byte("37.502669"), []byte("Catania")}, nil},
		{"GEOADD with options", "GEOADD", [][]byte{[]byte("geo3"), []byte("NX"), []byte("13.361389"), []byte("38.115556"), []byte("Palermo")}, nil},
		{"GEOPOS single", "GEOPOS", [][]byte{[]byte("geo4"), []byte("Palermo")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			s.Set("geo4", geo, store.SetOptions{})
		}},
		{"GEOPOS multiple", "GEOPOS", [][]byte{[]byte("geo5"), []byte("Palermo"), []byte("Catania")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo5", geo, store.SetOptions{})
		}},
		{"GEODIST", "GEODIST", [][]byte{[]byte("geo6"), []byte("Palermo"), []byte("Catania")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo6", geo, store.SetOptions{})
		}},
		{"GEODIST with unit", "GEODIST", [][]byte{[]byte("geo7"), []byte("Palermo"), []byte("Catania"), []byte("km")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo7", geo, store.SetOptions{})
		}},
		{"GEORADIUS", "GEORADIUS", [][]byte{[]byte("geo8"), []byte("15"), []byte("37"), []byte("200"), []byte("km")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo8", geo, store.SetOptions{})
		}},
		{"GEORADIUS with options", "GEORADIUS", [][]byte{[]byte("geo9"), []byte("15"), []byte("37"), []byte("200"), []byte("km"), []byte("WITHCOORD"), []byte("WITHDIST")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo9", geo, store.SetOptions{})
		}},
		{"GEORADIUSBYMEMBER", "GEORADIUSBYMEMBER", [][]byte{[]byte("geo10"), []byte("Palermo"), []byte("200"), []byte("km")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo10", geo, store.SetOptions{})
		}},
		{"GEOHASH", "GEOHASH", [][]byte{[]byte("geo11"), []byte("Palermo"), []byte("Catania")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo11", geo, store.SetOptions{})
		}},
		{"GEOSEARCH", "GEOSEARCH", [][]byte{[]byte("geo12"), []byte("FROMLONLAT"), []byte("15"), []byte("37"), []byte("BYRADIUS"), []byte("200"), []byte("km")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo12", geo, store.SetOptions{})
		}},
		{"GEOSEARCH BYBOX", "GEOSEARCH", [][]byte{[]byte("geo13"), []byte("FROMLONLAT"), []byte("15"), []byte("37"), []byte("BYBOX"), []byte("200"), []byte("200"), []byte("km")}, func() {
			geo := store.NewGeoValue()
			geo.Points["Palermo"] = store.GeoPoint{Lon: 13.361389, Lat: 38.115556}
			geo.Points["Catania"] = store.GeoPoint{Lon: 15.087269, Lat: 37.502669}
			s.Set("geo13", geo, store.SetOptions{})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			handler, ok := router.Get(tt.cmd)
			if !ok {
				t.Fatalf("Command %s not found", tt.cmd)
			}

			ctx := newTestContext(tt.cmd, tt.args, s)
			if err := handler.Handler(ctx); err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
