package search

import "sync/atomic"

// indexingOps counts, by field type, the field values indexed: RediSearch's
// total_indexing_ops_* of INFO search.
var indexingOps [FieldGeoShape + 1]atomic.Int64

func countIndexingOps(idx *Index, d *Document) {
	for i := range idx.Fields {
		f := d.field(i)
		if len(f.strs) == 0 && len(f.nums) == 0 && f.vec == nil {
			continue
		}
		if t := idx.Fields[i].Type; int(t) < len(indexingOps) {
			indexingOps[t].Add(1)
		}
	}
}

// IndexingOps returns how many field values of type t were indexed.
func IndexingOps(t FieldType) int64 {
	if int(t) >= len(indexingOps) {
		return 0
	}
	return indexingOps[t].Load()
}
