package store

import (
	"maps"
	"sort"
	"strings"
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

func (v *TimeSeriesValue) Lock()    { v.mu.Lock() }
func (v *TimeSeriesValue) Unlock()  { v.mu.Unlock() }
func (v *TimeSeriesValue) RLock()   { v.mu.RLock() }
func (v *TimeSeriesValue) RUnlock() { v.mu.RUnlock() }

func NewTimeSeriesValue(retention time.Duration) *TimeSeriesValue {
	return &TimeSeriesValue{
		Samples:   make([]TimeSeriesSample, 0),
		Labels:    make(map[string]string),
		Retention: retention,
	}
}

func (v *TimeSeriesValue) Type() DataType { return DataTypeTimeSeries }

func (v *TimeSeriesValue) SizeOf() int64 {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return int64(len(v.Samples)*24) + 64
}

func (v *TimeSeriesValue) String() string {
	return "timeseries"
}

func (v *TimeSeriesValue) Clone() Value {
	v.mu.RLock()
	defer v.mu.RUnlock()
	samples := make([]TimeSeriesSample, len(v.Samples))
	copy(samples, v.Samples)
	for i := range samples {
		samples[i].Labels = maps.Clone(samples[i].Labels)
	}
	return &TimeSeriesValue{
		Samples:   samples,
		Labels:    maps.Clone(v.Labels),
		Retention: v.Retention,
	}
}

type DuplicatePolicy int

const (
	DuplicateReject DuplicatePolicy = iota
	DuplicateFirst
	DuplicateLast
	DuplicateMin
	DuplicateMax
	DuplicateSum
)

func ParseDuplicatePolicy(policy string) (DuplicatePolicy, bool) {
	switch strings.ToUpper(policy) {
	case "BLOCK":
		return DuplicateReject, true
	case "FIRST":
		return DuplicateFirst, true
	case "LAST":
		return DuplicateLast, true
	case "MIN":
		return DuplicateMin, true
	case "MAX":
		return DuplicateMax, true
	case "SUM":
		return DuplicateSum, true
	}
	return DuplicateReject, false
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

func (v *TimeSeriesValue) AddWithPolicy(timestamp int64, value float64, policy DuplicatePolicy) (bool, int64) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if timestamp == 0 {
		timestamp = time.Now().UnixMilli()
	}

	for i := range v.Samples {
		if v.Samples[i].Timestamp != timestamp {
			continue
		}
		switch policy {
		case DuplicateFirst:
			return true, timestamp
		case DuplicateLast:
			v.Samples[i].Value = value
			return true, timestamp
		case DuplicateMin:
			if value < v.Samples[i].Value {
				v.Samples[i].Value = value
			}
			return true, timestamp
		case DuplicateMax:
			if value > v.Samples[i].Value {
				v.Samples[i].Value = value
			}
			return true, timestamp
		case DuplicateSum:
			v.Samples[i].Value += value
			return true, timestamp
		default:
			return false, timestamp
		}
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

	return true, timestamp
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

func (v *TimeSeriesValue) ReplaceLabels(labels map[string]string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.Labels = make(map[string]string, len(labels))
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
