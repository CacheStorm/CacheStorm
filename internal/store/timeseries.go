package store

import (
	"sort"
	"sync"
	"time"
)

type TimeSeriesSample struct {
	Timestamp int64
	Value     float64
	Labels    map[string]string
}

type TimeSeriesValue struct {
	Samples   []TimeSeriesSample
	Labels    map[string]string
	Retention time.Duration
	mu        sync.RWMutex
}

func NewTimeSeriesValue(retention time.Duration) *TimeSeriesValue {
	return &TimeSeriesValue{
		Samples:   make([]TimeSeriesSample, 0),
		Labels:    make(map[string]string),
		Retention: retention,
	}
}

func (v *TimeSeriesValue) Type() DataType { return DataTypeString }

func (v *TimeSeriesValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return int64(len(v.Samples)*24) + 64
}

func (v *TimeSeriesValue) String() string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return "timeseries"
}

func (v *TimeSeriesValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	samples := make([]TimeSeriesSample, len(v.Samples))
	copy(samples, v.Samples)
	return &TimeSeriesValue{
		Samples:   samples,
		Labels:    v.Labels,
		Retention: v.Retention,
	}
}

func (v *TimeSeriesValue) Add(timestamp int64, value float64) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()

	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}

	v.Samples = append(v.Samples, TimeSeriesSample{
		Timestamp: timestamp,
		Value:     value,
	})

	if v.Retention > 0 {
		cutoff := time.Now().Add(-v.Retention).UnixMilli()
		newSamples := make([]TimeSeriesSample, 0)
		for _, s := range v.Samples {
			if s.Timestamp >= cutoff {
				newSamples = append(newSamples, s)
			}
		}
		v.Samples = newSamples
	}

	return timestamp
}

func (v *TimeSeriesValue) AddWithLabels(timestamp int64, value float64, labels map[string]string) int64 {
	v.mu.Lock()
	defer v.mu.Unlock()

	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}

	v.Samples = append(v.Samples, TimeSeriesSample{
		Timestamp: timestamp,
		Value:     value,
		Labels:    labels,
	})

	for k, val := range labels {
		v.Labels[k] = val
	}

	return timestamp
}

func (v *TimeSeriesValue) Range(from, to int64) []TimeSeriesSample {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]TimeSeriesSample, 0)
	for _, s := range v.Samples {
		if s.Timestamp >= from && s.Timestamp <= to {
			result = append(result, s)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result
}

func (v *TimeSeriesValue) RangeWithCount(from, to int64, count int) []TimeSeriesSample {
	samples := v.Range(from, to)
	if count > 0 && len(samples) > count {
		return samples[:count]
	}
	return samples
}

func (v *TimeSeriesValue) Get(timestamp int64) *TimeSeriesSample {
	v.mu.RLock()
	defer v.mu.RUnlock()

	for _, s := range v.Samples {
		if s.Timestamp == timestamp {
			return &s
		}
	}
	return nil
}

func (v *TimeSeriesValue) Latest() *TimeSeriesSample {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.Samples) == 0 {
		return nil
	}

	var latest *TimeSeriesSample
	for i := range v.Samples {
		if latest == nil || v.Samples[i].Timestamp > latest.Timestamp {
			latest = &v.Samples[i]
		}
	}
	return latest
}

func (v *TimeSeriesValue) First() *TimeSeriesSample {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.Samples) == 0 {
		return nil
	}

	var first *TimeSeriesSample
	for i := range v.Samples {
		if first == nil || v.Samples[i].Timestamp < first.Timestamp {
			first = &v.Samples[i]
		}
	}
	return first
}

func (v *TimeSeriesValue) Delete(timestamp int64) int {
	v.mu.Lock()
	defer v.mu.Unlock()

	newSamples := make([]TimeSeriesSample, 0)
	deleted := 0

	for _, s := range v.Samples {
		if s.Timestamp == timestamp {
			deleted++
		} else {
			newSamples = append(newSamples, s)
		}
	}

	v.Samples = newSamples
	return deleted
}

func (v *TimeSeriesValue) Len() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.Samples)
}

func (v *TimeSeriesValue) SetRetention(retention time.Duration) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Retention = retention
}

func (v *TimeSeriesValue) SetLabels(labels map[string]string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for k, val := range labels {
		v.Labels[k] = val
	}
}

func (v *TimeSeriesValue) GetLabels() map[string]string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	result := make(map[string]string, len(v.Labels))
	for k, val := range v.Labels {
		result[k] = val
	}
	return result
}

func (v *TimeSeriesValue) Aggregation(from, to int64, aggType string, bucketSize int64) []TimeSeriesSample {
	samples := v.Range(from, to)
	if len(samples) == 0 {
		return nil
	}

	result := make([]TimeSeriesSample, 0)
	buckets := make(map[int64][]TimeSeriesSample)

	for _, s := range samples {
		bucket := s.Timestamp / bucketSize * bucketSize
		buckets[bucket] = append(buckets[bucket], s)
	}

	for bucket, bucketSamples := range buckets {
		var aggValue float64
		switch aggType {
		case "avg":
			sum := 0.0
			for _, s := range bucketSamples {
				sum += s.Value
			}
			aggValue = sum / float64(len(bucketSamples))
		case "sum":
			for _, s := range bucketSamples {
				aggValue += s.Value
			}
		case "min":
			aggValue = bucketSamples[0].Value
			for _, s := range bucketSamples {
				if s.Value < aggValue {
					aggValue = s.Value
				}
			}
		case "max":
			aggValue = bucketSamples[0].Value
			for _, s := range bucketSamples {
				if s.Value > aggValue {
					aggValue = s.Value
				}
			}
		case "count":
			aggValue = float64(len(bucketSamples))
		case "first":
			aggValue = bucketSamples[0].Value
		case "last":
			aggValue = bucketSamples[len(bucketSamples)-1].Value
		default:
			aggValue = bucketSamples[0].Value
		}

		result = append(result, TimeSeriesSample{
			Timestamp: bucket,
			Value:     aggValue,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp < result[j].Timestamp
	})

	return result
}

type TimeSeriesManager struct {
	mu      sync.RWMutex
	series  map[string]*TimeSeriesValue
	byLabel map[string]map[string][]string
}

func NewTimeSeriesManager() *TimeSeriesManager {
	return &TimeSeriesManager{
		series:  make(map[string]*TimeSeriesValue),
		byLabel: make(map[string]map[string][]string),
	}
}

func (m *TimeSeriesManager) Create(key string, retention time.Duration, labels map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.series[key]; exists {
		return nil
	}

	ts := NewTimeSeriesValue(retention)
	ts.Labels = labels
	m.series[key] = ts

	for k, v := range labels {
		if m.byLabel[k] == nil {
			m.byLabel[k] = make(map[string][]string)
		}
		m.byLabel[k][v] = append(m.byLabel[k][v], key)
	}

	return nil
}

func (m *TimeSeriesManager) Get(key string) (*TimeSeriesValue, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ts, ok := m.series[key]
	return ts, ok
}

func (m *TimeSeriesManager) Delete(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ts, exists := m.series[key]
	if !exists {
		return false
	}

	for k, v := range ts.Labels {
		if keys, ok := m.byLabel[k][v]; ok {
			newKeys := make([]string, 0)
			for _, k2 := range keys {
				if k2 != key {
					newKeys = append(newKeys, k2)
				}
			}
			m.byLabel[k][v] = newKeys
		}
	}

	delete(m.series, key)
	return true
}

func (m *TimeSeriesManager) QueryByLabels(labels map[string]string, filter string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, 0)
	for key, ts := range m.series {
		match := true
		for k, v := range labels {
			if ts.Labels[k] != v {
				match = false
				break
			}
		}
		if match {
			result = append(result, key)
		}
	}

	return result
}
