package command

import (
	"testing"

	"github.com/cachestorm/cachestorm/internal/store"
)

func barg(s string) []byte { return []byte(s) }

// ---------------------------------------------------------------------------
// SET commands
// ---------------------------------------------------------------------------

func TestSADD_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SADD", [][]byte{barg("k")}, s)
		if err := cmdSADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add single member", func(t *testing.T) {
		ctx := discardCtx("SADD", [][]byte{barg("s1"), barg("a")}, s)
		if err := cmdSADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add duplicate member", func(t *testing.T) {
		ctx := discardCtx("SADD", [][]byte{barg("s1"), barg("a")}, s)
		if err := cmdSADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add multiple members", func(t *testing.T) {
		ctx := discardCtx("SADD", [][]byte{barg("s1"), barg("b"), barg("c"), barg("d")}, s)
		if err := cmdSADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("strkey", &store.StringValue{Data: []byte("hi")}, store.SetOptions{})
		ctx := discardCtx("SADD", [][]byte{barg("strkey"), barg("m")}, s)
		if err := cmdSADD(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSREM_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SREM", [][]byte{barg("k")}, s)
		if err := cmdSREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("SREM", [][]byte{barg("nosuch"), barg("a")}, s)
		if err := cmdSREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("remove existing member", func(t *testing.T) {
		s.Set("rs1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SREM", [][]byte{barg("rs1"), barg("a")}, s)
		if err := cmdSREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("remove non-existing member", func(t *testing.T) {
		s.Set("rs2", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		ctx := discardCtx("SREM", [][]byte{barg("rs2"), barg("z")}, s)
		if err := cmdSREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("remove all members deletes key", func(t *testing.T) {
		s.Set("rs3", &store.SetValue{Members: map[string]struct{}{"x": {}}}, store.SetOptions{})
		ctx := discardCtx("SREM", [][]byte{barg("rs3"), barg("x")}, s)
		if err := cmdSREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("strrem", &store.StringValue{Data: []byte("hi")}, store.SetOptions{})
		ctx := discardCtx("SREM", [][]byte{barg("strrem"), barg("a")}, s)
		if err := cmdSREM(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSMEMBERS_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("SMEMBERS", [][]byte{}, s)
		if err := cmdSMEMBERS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("SMEMBERS", [][]byte{barg("nosuch")}, s)
		if err := cmdSMEMBERS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("existing set", func(t *testing.T) {
		s.Set("sm1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SMEMBERS", [][]byte{barg("sm1")}, s)
		if err := cmdSMEMBERS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("smwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SMEMBERS", [][]byte{barg("smwt")}, s)
		if err := cmdSMEMBERS(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSISMEMBER_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("SISMEMBER", [][]byte{barg("k")}, s)
		if err := cmdSISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("SISMEMBER", [][]byte{barg("nosuch"), barg("m")}, s)
		if err := cmdSISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member exists", func(t *testing.T) {
		s.Set("si1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SISMEMBER", [][]byte{barg("si1"), barg("a")}, s)
		if err := cmdSISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member not exists", func(t *testing.T) {
		ctx := discardCtx("SISMEMBER", [][]byte{barg("si1"), barg("z")}, s)
		if err := cmdSISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("siwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SISMEMBER", [][]byte{barg("siwt"), barg("a")}, s)
		if err := cmdSISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSCARD_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("SCARD", [][]byte{}, s)
		if err := cmdSCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("SCARD", [][]byte{barg("nosuch")}, s)
		if err := cmdSCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("existing set", func(t *testing.T) {
		s.Set("sc1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		ctx := discardCtx("SCARD", [][]byte{barg("sc1")}, s)
		if err := cmdSCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("scwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SCARD", [][]byte{barg("scwt")}, s)
		if err := cmdSCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSPOP_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SPOP", [][]byte{}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found single", func(t *testing.T) {
		ctx := discardCtx("SPOP", [][]byte{barg("nosuch")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found with count", func(t *testing.T) {
		ctx := discardCtx("SPOP", [][]byte{barg("nosuch"), barg("3")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop single", func(t *testing.T) {
		s.Set("sp1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SPOP", [][]byte{barg("sp1")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop with count less than set size", func(t *testing.T) {
		s.Set("sp2", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		ctx := discardCtx("SPOP", [][]byte{barg("sp2"), barg("2")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop with count >= set size", func(t *testing.T) {
		s.Set("sp3", &store.SetValue{Members: map[string]struct{}{"x": {}, "y": {}}}, store.SetOptions{})
		ctx := discardCtx("SPOP", [][]byte{barg("sp3"), barg("10")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop last member deletes key", func(t *testing.T) {
		s.Set("sp4", &store.SetValue{Members: map[string]struct{}{"only": {}}}, store.SetOptions{})
		ctx := discardCtx("SPOP", [][]byte{barg("sp4")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid count", func(t *testing.T) {
		ctx := discardCtx("SPOP", [][]byte{barg("sp1"), barg("abc")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("spwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SPOP", [][]byte{barg("spwt")}, s)
		if err := cmdSPOP(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSRANDMEMBER_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SRANDMEMBER", [][]byte{}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found no count", func(t *testing.T) {
		ctx := discardCtx("SRANDMEMBER", [][]byte{barg("nosuch")}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found with count", func(t *testing.T) {
		ctx := discardCtx("SRANDMEMBER", [][]byte{barg("nosuch"), barg("3")}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("single member no count", func(t *testing.T) {
		s.Set("sr1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SRANDMEMBER", [][]byte{barg("sr1")}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("count larger than set", func(t *testing.T) {
		s.Set("sr2", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SRANDMEMBER", [][]byte{barg("sr2"), barg("10")}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("count smaller than set", func(t *testing.T) {
		s.Set("sr3", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		ctx := discardCtx("SRANDMEMBER", [][]byte{barg("sr3"), barg("2")}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid count", func(t *testing.T) {
		ctx := discardCtx("SRANDMEMBER", [][]byte{barg("sr3"), barg("abc")}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("srwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SRANDMEMBER", [][]byte{barg("srwt")}, s)
		if err := cmdSRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSMOVE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("SMOVE", [][]byte{barg("a"), barg("b")}, s)
		if err := cmdSMOVE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("src not found", func(t *testing.T) {
		ctx := discardCtx("SMOVE", [][]byte{barg("nosrc"), barg("dst"), barg("m")}, s)
		if err := cmdSMOVE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member not in src", func(t *testing.T) {
		s.Set("mv1", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		ctx := discardCtx("SMOVE", [][]byte{barg("mv1"), barg("mvdst"), barg("z")}, s)
		if err := cmdSMOVE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("successful move", func(t *testing.T) {
		s.Set("mv2", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		s.Set("mv2dst", &store.SetValue{Members: map[string]struct{}{"c": {}}}, store.SetOptions{})
		ctx := discardCtx("SMOVE", [][]byte{barg("mv2"), barg("mv2dst"), barg("a")}, s)
		if err := cmdSMOVE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("move last member deletes src", func(t *testing.T) {
		s.Set("mv3", &store.SetValue{Members: map[string]struct{}{"only": {}}}, store.SetOptions{})
		ctx := discardCtx("SMOVE", [][]byte{barg("mv3"), barg("mv3dst"), barg("only")}, s)
		if err := cmdSMOVE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type src", func(t *testing.T) {
		s.Set("mvwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SMOVE", [][]byte{barg("mvwt"), barg("dst"), barg("m")}, s)
		if err := cmdSMOVE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSUNION_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SUNION", [][]byte{}, s)
		if err := cmdSUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("single set", func(t *testing.T) {
		s.Set("su1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SUNION", [][]byte{barg("su1")}, s)
		if err := cmdSUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("two sets", func(t *testing.T) {
		s.Set("su2a", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		s.Set("su2b", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}}}, store.SetOptions{})
		ctx := discardCtx("SUNION", [][]byte{barg("su2a"), barg("su2b")}, s)
		if err := cmdSUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("one key missing", func(t *testing.T) {
		s.Set("su3", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		ctx := discardCtx("SUNION", [][]byte{barg("su3"), barg("nosuch")}, s)
		if err := cmdSUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("suwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SUNION", [][]byte{barg("suwt")}, s)
		if err := cmdSUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSINTER_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SINTER", [][]byte{}, s)
		if err := cmdSINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("two sets with overlap", func(t *testing.T) {
		s.Set("si2a", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		s.Set("si2b", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTER", [][]byte{barg("si2a"), barg("si2b")}, s)
		if err := cmdSINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("one key missing returns empty", func(t *testing.T) {
		s.Set("si3", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTER", [][]byte{barg("si3"), barg("nosuch")}, s)
		if err := cmdSINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("siwt2", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SINTER", [][]byte{barg("siwt2")}, s)
		if err := cmdSINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSDIFF_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SDIFF", [][]byte{}, s)
		if err := cmdSDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("two sets", func(t *testing.T) {
		s.Set("sd1a", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		s.Set("sd1b", &store.SetValue{Members: map[string]struct{}{"b": {}, "d": {}}}, store.SetOptions{})
		ctx := discardCtx("SDIFF", [][]byte{barg("sd1a"), barg("sd1b")}, s)
		if err := cmdSDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("first key missing", func(t *testing.T) {
		ctx := discardCtx("SDIFF", [][]byte{barg("nosuch"), barg("sd1b")}, s)
		if err := cmdSDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("sdwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SDIFF", [][]byte{barg("sdwt")}, s)
		if err := cmdSDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSUNIONSTORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SUNIONSTORE", [][]byte{barg("dst")}, s)
		if err := cmdSUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("store union", func(t *testing.T) {
		s.Set("sus1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		s.Set("sus2", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}}}, store.SetOptions{})
		ctx := discardCtx("SUNIONSTORE", [][]byte{barg("susdst"), barg("sus1"), barg("sus2")}, s)
		if err := cmdSUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty result deletes dest", func(t *testing.T) {
		ctx := discardCtx("SUNIONSTORE", [][]byte{barg("susdst2"), barg("nosuch1"), barg("nosuch2")}, s)
		if err := cmdSUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSINTERSTORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SINTERSTORE", [][]byte{barg("dst")}, s)
		if err := cmdSINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("store intersection", func(t *testing.T) {
		s.Set("sis1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		s.Set("sis2", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTERSTORE", [][]byte{barg("sisdst"), barg("sis1"), barg("sis2")}, s)
		if err := cmdSINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty intersection", func(t *testing.T) {
		s.Set("sis3", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTERSTORE", [][]byte{barg("sisdst2"), barg("sis3"), barg("nosuch")}, s)
		if err := cmdSINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("no overlap empty result", func(t *testing.T) {
		s.Set("sis4a", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		s.Set("sis4b", &store.SetValue{Members: map[string]struct{}{"z": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTERSTORE", [][]byte{barg("sisdst3"), barg("sis4a"), barg("sis4b")}, s)
		if err := cmdSINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSDIFFSTORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SDIFFSTORE", [][]byte{barg("dst")}, s)
		if err := cmdSDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("store diff", func(t *testing.T) {
		s.Set("sds1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		s.Set("sds2", &store.SetValue{Members: map[string]struct{}{"b": {}}}, store.SetOptions{})
		ctx := discardCtx("SDIFFSTORE", [][]byte{barg("sdsdst"), barg("sds1"), barg("sds2")}, s)
		if err := cmdSDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty diff result", func(t *testing.T) {
		s.Set("sds3", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		s.Set("sds4", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		ctx := discardCtx("SDIFFSTORE", [][]byte{barg("sdsdst2"), barg("sds3"), barg("sds4")}, s)
		if err := cmdSDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSSCAN_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("k")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid cursor", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("k"), barg("abc")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("nosuch"), barg("0")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with defaults", func(t *testing.T) {
		s.Set("ss1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		ctx := discardCtx("SSCAN", [][]byte{barg("ss1"), barg("0")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with MATCH", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("ss1"), barg("0"), barg("MATCH"), barg("*")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with COUNT", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("ss1"), barg("0"), barg("COUNT"), barg("2")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with MATCH missing value", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("ss1"), barg("0"), barg("MATCH")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with COUNT missing value", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("ss1"), barg("0"), barg("COUNT")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with invalid COUNT", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("ss1"), barg("0"), barg("COUNT"), barg("abc")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with unknown option", func(t *testing.T) {
		ctx := discardCtx("SSCAN", [][]byte{barg("ss1"), barg("0"), barg("BADOPT")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("sswt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SSCAN", [][]byte{barg("sswt"), barg("0")}, s)
		if err := cmdSSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSINTERCARD_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SINTERCARD", [][]byte{}, s)
		if err := cmdSINTERCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("single key", func(t *testing.T) {
		s.Set("sic1", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTERCARD", [][]byte{barg("sic1")}, s)
		if err := cmdSINTERCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("two keys with overlap", func(t *testing.T) {
		s.Set("sic2a", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		s.Set("sic2b", &store.SetValue{Members: map[string]struct{}{"b": {}, "c": {}, "d": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTERCARD", [][]byte{barg("sic2a"), barg("sic2b")}, s)
		if err := cmdSINTERCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with LIMIT", func(t *testing.T) {
		s.Set("sic3a", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		s.Set("sic3b", &store.SetValue{Members: map[string]struct{}{"a": {}, "b": {}, "c": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTERCARD", [][]byte{barg("sic3a"), barg("sic3b"), barg("LIMIT"), barg("1")}, s)
		if err := cmdSINTERCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with invalid LIMIT", func(t *testing.T) {
		ctx := discardCtx("SINTERCARD", [][]byte{barg("sic3a"), barg("sic3b"), barg("LIMIT"), barg("abc")}, s)
		if err := cmdSINTERCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("one key missing returns 0", func(t *testing.T) {
		s.Set("sic4", &store.SetValue{Members: map[string]struct{}{"a": {}}}, store.SetOptions{})
		ctx := discardCtx("SINTERCARD", [][]byte{barg("sic4"), barg("nosuch")}, s)
		if err := cmdSINTERCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestSMISMEMBER_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("SMISMEMBER", [][]byte{barg("k")}, s)
		if err := cmdSMISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("SMISMEMBER", [][]byte{barg("nosuch"), barg("a"), barg("b")}, s)
		if err := cmdSMISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("existing set mixed results", func(t *testing.T) {
		s.Set("smis1", &store.SetValue{Members: map[string]struct{}{"a": {}, "c": {}}}, store.SetOptions{})
		ctx := discardCtx("SMISMEMBER", [][]byte{barg("smis1"), barg("a"), barg("b"), barg("c")}, s)
		if err := cmdSMISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("smiswt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("SMISMEMBER", [][]byte{barg("smiswt"), barg("a")}, s)
		if err := cmdSMISMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

// ---------------------------------------------------------------------------
// SORTED SET commands
// ---------------------------------------------------------------------------

func TestZADD_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("k"), barg("1")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add single", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z1"), barg("1.5"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add multiple", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z2"), barg("1"), barg("a"), barg("2"), barg("b"), barg("3"), barg("c")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("NX flag skip existing", func(t *testing.T) {
		s.Set("z3", &store.SortedSetValue{Members: map[string]float64{"a": 1.0}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z3"), barg("NX"), barg("5"), barg("a"), barg("2"), barg("b")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("XX flag skip new", func(t *testing.T) {
		s.Set("z4", &store.SortedSetValue{Members: map[string]float64{"a": 1.0}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z4"), barg("XX"), barg("5"), barg("a"), barg("2"), barg("newmem")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("NX and XX conflict", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z5"), barg("NX"), barg("XX"), barg("1"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("GT flag", func(t *testing.T) {
		s.Set("z6", &store.SortedSetValue{Members: map[string]float64{"a": 5.0}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z6"), barg("GT"), barg("3"), barg("a"), barg("10"), barg("b")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("LT flag", func(t *testing.T) {
		s.Set("z7", &store.SortedSetValue{Members: map[string]float64{"a": 5.0}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z7"), barg("LT"), barg("3"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("CH flag", func(t *testing.T) {
		s.Set("z8", &store.SortedSetValue{Members: map[string]float64{"a": 1.0}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z8"), barg("CH"), barg("5"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("INCR flag", func(t *testing.T) {
		s.Set("z9", &store.SortedSetValue{Members: map[string]float64{"a": 1.0}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z9"), barg("INCR"), barg("5"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("INCR on new member", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z10"), barg("INCR"), barg("3"), barg("newm")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("INCR with XX on missing member", func(t *testing.T) {
		s.Set("z11", &store.SortedSetValue{Members: map[string]float64{}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z11"), barg("XX"), barg("INCR"), barg("5"), barg("missing")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid score", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z12"), barg("abc"), barg("m")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("odd score/member pairs", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z13"), barg("1")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("INCR with multiple pairs error", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z14"), barg("INCR"), barg("1"), barg("a"), barg("2"), barg("b")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("zwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("zwt"), barg("1"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("GT and LT conflict", func(t *testing.T) {
		ctx := discardCtx("ZADD", [][]byte{barg("z15"), barg("GT"), barg("LT"), barg("1"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("XX on empty set", func(t *testing.T) {
		s.Set("z16", &store.SortedSetValue{Members: map[string]float64{}}, store.SetOptions{})
		ctx := discardCtx("ZADD", [][]byte{barg("z16"), barg("XX"), barg("1"), barg("a")}, s)
		if err := cmdZADD(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZREM_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZREM", [][]byte{barg("k")}, s)
		if err := cmdZREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZREM", [][]byte{barg("nosuch"), barg("m")}, s)
		if err := cmdZREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("remove existing", func(t *testing.T) {
		s.Set("zr1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2}}, store.SetOptions{})
		ctx := discardCtx("ZREM", [][]byte{barg("zr1"), barg("a")}, s)
		if err := cmdZREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("remove all empties key", func(t *testing.T) {
		s.Set("zr2", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		ctx := discardCtx("ZREM", [][]byte{barg("zr2"), barg("a")}, s)
		if err := cmdZREM(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("zrwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("ZREM", [][]byte{barg("zrwt"), barg("a")}, s)
		if err := cmdZREM(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZSCORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("ZSCORE", [][]byte{barg("k")}, s)
		if err := cmdZSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZSCORE", [][]byte{barg("nosuch"), barg("m")}, s)
		if err := cmdZSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member exists", func(t *testing.T) {
		s.Set("zs1", &store.SortedSetValue{Members: map[string]float64{"a": 3.14}}, store.SetOptions{})
		ctx := discardCtx("ZSCORE", [][]byte{barg("zs1"), barg("a")}, s)
		if err := cmdZSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member not found", func(t *testing.T) {
		ctx := discardCtx("ZSCORE", [][]byte{barg("zs1"), barg("nosuch")}, s)
		if err := cmdZSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("zswt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("ZSCORE", [][]byte{barg("zswt"), barg("m")}, s)
		if err := cmdZSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZRANK_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("ZRANK", [][]byte{barg("k")}, s)
		if err := cmdZRANK(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZRANK", [][]byte{barg("nosuch"), barg("m")}, s)
		if err := cmdZRANK(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member exists", func(t *testing.T) {
		s.Set("zrk1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZRANK", [][]byte{barg("zrk1"), barg("b")}, s)
		if err := cmdZRANK(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member not found", func(t *testing.T) {
		ctx := discardCtx("ZRANK", [][]byte{barg("zrk1"), barg("z")}, s)
		if err := cmdZRANK(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZCARD_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("ZCARD", [][]byte{}, s)
		if err := cmdZCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZCARD", [][]byte{barg("nosuch")}, s)
		if err := cmdZCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("existing zset", func(t *testing.T) {
		s.Set("zc1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2}}, store.SetOptions{})
		ctx := discardCtx("ZCARD", [][]byte{barg("zc1")}, s)
		if err := cmdZCARD(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZCOUNT_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("ZCOUNT", [][]byte{barg("k"), barg("1")}, s)
		if err := cmdZCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid min", func(t *testing.T) {
		ctx := discardCtx("ZCOUNT", [][]byte{barg("k"), barg("abc"), barg("5")}, s)
		if err := cmdZCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid max", func(t *testing.T) {
		ctx := discardCtx("ZCOUNT", [][]byte{barg("k"), barg("1"), barg("abc")}, s)
		if err := cmdZCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZCOUNT", [][]byte{barg("nosuch"), barg("0"), barg("10")}, s)
		if err := cmdZCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("count in range", func(t *testing.T) {
		s.Set("zcnt1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 5, "c": 10}}, store.SetOptions{})
		ctx := discardCtx("ZCOUNT", [][]byte{barg("zcnt1"), barg("0"), barg("6")}, s)
		if err := cmdZCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZINCRBY_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("ZINCRBY", [][]byte{barg("k"), barg("1")}, s)
		if err := cmdZINCRBY(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid incr", func(t *testing.T) {
		ctx := discardCtx("ZINCRBY", [][]byte{barg("k"), barg("abc"), barg("m")}, s)
		if err := cmdZINCRBY(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("new member", func(t *testing.T) {
		ctx := discardCtx("ZINCRBY", [][]byte{barg("zi1"), barg("5"), barg("m")}, s)
		if err := cmdZINCRBY(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("existing member", func(t *testing.T) {
		s.Set("zi2", &store.SortedSetValue{Members: map[string]float64{"a": 10}}, store.SetOptions{})
		ctx := discardCtx("ZINCRBY", [][]byte{barg("zi2"), barg("3"), barg("a")}, s)
		if err := cmdZINCRBY(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZRANGEBYSCORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYSCORE", [][]byte{barg("k"), barg("0")}, s)
		if err := cmdZRANGEBYSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid min", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYSCORE", [][]byte{barg("k"), barg("abc"), barg("5")}, s)
		if err := cmdZRANGEBYSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid max", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYSCORE", [][]byte{barg("k"), barg("0"), barg("abc")}, s)
		if err := cmdZRANGEBYSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYSCORE", [][]byte{barg("nosuch"), barg("0"), barg("10")}, s)
		if err := cmdZRANGEBYSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("success", func(t *testing.T) {
		s.Set("zrbs1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 5, "c": 10}}, store.SetOptions{})
		ctx := discardCtx("ZRANGEBYSCORE", [][]byte{barg("zrbs1"), barg("0"), barg("6")}, s)
		if err := cmdZRANGEBYSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WITHSCORES", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYSCORE", [][]byte{barg("zrbs1"), barg("0"), barg("100"), barg("WITHSCORES")}, s)
		if err := cmdZRANGEBYSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZRANGEBYLEX_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("k"), barg("[a")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("nosuch"), barg("[a"), barg("[z")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("success", func(t *testing.T) {
		s.Set("zrbl1", &store.SortedSetValue{Members: map[string]float64{"a": 0, "b": 0, "c": 0, "d": 0}}, store.SetOptions{})
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("zrbl1"), barg("-"), barg("+")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with LIMIT", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("zrbl1"), barg("[a"), barg("[z"), barg("LIMIT"), barg("0"), barg("2")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with REV", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("zrbl1"), barg("[a"), barg("[z"), barg("REV")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("LIMIT invalid offset", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("zrbl1"), barg("[a"), barg("[z"), barg("LIMIT"), barg("abc"), barg("2")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("LIMIT invalid count", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("zrbl1"), barg("[a"), barg("[z"), barg("LIMIT"), barg("0"), barg("abc")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("LIMIT missing args", func(t *testing.T) {
		ctx := discardCtx("ZRANGEBYLEX", [][]byte{barg("zrbl1"), barg("[a"), barg("[z"), barg("LIMIT")}, s)
		if err := cmdZRANGEBYLEX(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZRANGE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("k"), barg("0")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("nosuch"), barg("0"), barg("-1")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("by index", func(t *testing.T) {
		s.Set("zrng1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("0"), barg("-1")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WITHSCORES", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("0"), barg("-1"), barg("WITHSCORES")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with REV", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("0"), barg("-1"), barg("REV")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYSCORE", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("1"), barg("3"), barg("BYSCORE")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYSCORE with LIMIT", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("1"), barg("3"), barg("BYSCORE"), barg("LIMIT"), barg("0"), barg("1")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYLEX", func(t *testing.T) {
		s.Set("zrng2", &store.SortedSetValue{Members: map[string]float64{"a": 0, "b": 0, "c": 0}}, store.SetOptions{})
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng2"), barg("[a"), barg("[c"), barg("BYLEX")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYLEX with LIMIT", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng2"), barg("[a"), barg("[z"), barg("BYLEX"), barg("LIMIT"), barg("0"), barg("2")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid start index", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("abc"), barg("1")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid stop index", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("0"), barg("abc")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("LIMIT missing args", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("1"), barg("3"), barg("BYSCORE"), barg("LIMIT")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("LIMIT invalid offset", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("1"), barg("3"), barg("BYSCORE"), barg("LIMIT"), barg("abc"), barg("1")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYSCORE with exclusive ranges", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("(1"), barg("(3"), barg("BYSCORE")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYSCORE with inf", func(t *testing.T) {
		ctx := discardCtx("ZRANGE", [][]byte{barg("zrng1"), barg("-inf"), barg("+inf"), barg("BYSCORE")}, s)
		if err := cmdZRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZREVRANGE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZREVRANGE", [][]byte{barg("k"), barg("0")}, s)
		if err := cmdZREVRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid start", func(t *testing.T) {
		ctx := discardCtx("ZREVRANGE", [][]byte{barg("k"), barg("abc"), barg("0")}, s)
		if err := cmdZREVRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid stop", func(t *testing.T) {
		ctx := discardCtx("ZREVRANGE", [][]byte{barg("k"), barg("0"), barg("abc")}, s)
		if err := cmdZREVRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZREVRANGE", [][]byte{barg("nosuch"), barg("0"), barg("-1")}, s)
		if err := cmdZREVRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("success", func(t *testing.T) {
		s.Set("zrv1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZREVRANGE", [][]byte{barg("zrv1"), barg("0"), barg("-1")}, s)
		if err := cmdZREVRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WITHSCORES", func(t *testing.T) {
		ctx := discardCtx("ZREVRANGE", [][]byte{barg("zrv1"), barg("0"), barg("-1"), barg("WITHSCORES")}, s)
		if err := cmdZREVRANGE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZPOPMIN_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZPOPMIN", [][]byte{}, s)
		if err := cmdZPOPMIN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZPOPMIN", [][]byte{barg("nosuch")}, s)
		if err := cmdZPOPMIN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop single", func(t *testing.T) {
		s.Set("zpm1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZPOPMIN", [][]byte{barg("zpm1")}, s)
		if err := cmdZPOPMIN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop with count", func(t *testing.T) {
		s.Set("zpm2", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZPOPMIN", [][]byte{barg("zpm2"), barg("2")}, s)
		if err := cmdZPOPMIN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid count", func(t *testing.T) {
		ctx := discardCtx("ZPOPMIN", [][]byte{barg("zpm1"), barg("abc")}, s)
		if err := cmdZPOPMIN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty zset", func(t *testing.T) {
		s.Set("zpm3", &store.SortedSetValue{Members: map[string]float64{}}, store.SetOptions{})
		ctx := discardCtx("ZPOPMIN", [][]byte{barg("zpm3")}, s)
		if err := cmdZPOPMIN(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZPOPMAX_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZPOPMAX", [][]byte{}, s)
		if err := cmdZPOPMAX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZPOPMAX", [][]byte{barg("nosuch")}, s)
		if err := cmdZPOPMAX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop single", func(t *testing.T) {
		s.Set("zpx1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZPOPMAX", [][]byte{barg("zpx1")}, s)
		if err := cmdZPOPMAX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("pop with count", func(t *testing.T) {
		s.Set("zpx2", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZPOPMAX", [][]byte{barg("zpx2"), barg("2")}, s)
		if err := cmdZPOPMAX(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid count", func(t *testing.T) {
		ctx := discardCtx("ZPOPMAX", [][]byte{barg("zpx1"), barg("abc")}, s)
		if err := cmdZPOPMAX(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZRANGESTORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZRANGESTORE", [][]byte{barg("dst"), barg("src"), barg("0")}, s)
		if err := cmdZRANGESTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid start", func(t *testing.T) {
		ctx := discardCtx("ZRANGESTORE", [][]byte{barg("dst"), barg("src"), barg("abc"), barg("1")}, s)
		if err := cmdZRANGESTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid stop", func(t *testing.T) {
		ctx := discardCtx("ZRANGESTORE", [][]byte{barg("dst"), barg("src"), barg("0"), barg("abc")}, s)
		if err := cmdZRANGESTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("src not found", func(t *testing.T) {
		ctx := discardCtx("ZRANGESTORE", [][]byte{barg("dst"), barg("nosuch"), barg("0"), barg("-1")}, s)
		if err := cmdZRANGESTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("success", func(t *testing.T) {
		s.Set("zrss1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZRANGESTORE", [][]byte{barg("zrsdst"), barg("zrss1"), barg("0"), barg("-1")}, s)
		if err := cmdZRANGESTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with REV", func(t *testing.T) {
		ctx := discardCtx("ZRANGESTORE", [][]byte{barg("zrsdst2"), barg("zrss1"), barg("0"), barg("-1"), barg("REV")}, s)
		if err := cmdZRANGESTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		s.Set("zrss2", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		ctx := discardCtx("ZRANGESTORE", [][]byte{barg("zrsdst3"), barg("zrss2"), barg("5"), barg("10")}, s)
		if err := cmdZRANGESTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZLEXCOUNT_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("wrong arg count", func(t *testing.T) {
		ctx := discardCtx("ZLEXCOUNT", [][]byte{barg("k"), barg("[a")}, s)
		if err := cmdZLEXCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZLEXCOUNT", [][]byte{barg("nosuch"), barg("[a"), barg("[z")}, s)
		if err := cmdZLEXCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("count", func(t *testing.T) {
		s.Set("zlc1", &store.SortedSetValue{Members: map[string]float64{"a": 0, "b": 0, "c": 0}}, store.SetOptions{})
		ctx := discardCtx("ZLEXCOUNT", [][]byte{barg("zlc1"), barg("-"), barg("+")}, s)
		if err := cmdZLEXCOUNT(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZMSCORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZMSCORE", [][]byte{barg("k")}, s)
		if err := cmdZMSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZMSCORE", [][]byte{barg("nosuch"), barg("a"), barg("b")}, s)
		if err := cmdZMSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("mixed existing and missing", func(t *testing.T) {
		s.Set("zms1", &store.SortedSetValue{Members: map[string]float64{"a": 1.5, "c": 3.5}}, store.SetOptions{})
		ctx := discardCtx("ZMSCORE", [][]byte{barg("zms1"), barg("a"), barg("b"), barg("c")}, s)
		if err := cmdZMSCORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZRANDMEMBER_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("nosuch")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found with count 0", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("nosuch"), barg("COUNT"), barg("0")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("positive count", func(t *testing.T) {
		s.Set("zrm1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("zrm1"), barg("COUNT"), barg("2")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("positive count larger than set", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("zrm1"), barg("COUNT"), barg("10")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("negative count (allow repeats)", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("zrm1"), barg("COUNT"), barg("-3")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WITHSCORES", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("zrm1"), barg("COUNT"), barg("2"), barg("WITHSCORES")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid count", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("zrm1"), barg("COUNT"), barg("abc")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("COUNT missing value", func(t *testing.T) {
		ctx := discardCtx("ZRANDMEMBER", [][]byte{barg("zrm1"), barg("COUNT")}, s)
		if err := cmdZRANDMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZSCAN_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("k")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid cursor", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("k"), barg("abc")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("nosuch"), barg("0")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with defaults", func(t *testing.T) {
		s.Set("zsc1", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		ctx := discardCtx("ZSCAN", [][]byte{barg("zsc1"), barg("0")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with MATCH", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("zsc1"), barg("0"), barg("MATCH"), barg("*")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("scan with COUNT", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("zsc1"), barg("0"), barg("COUNT"), barg("2")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("MATCH missing value", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("zsc1"), barg("0"), barg("MATCH")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("COUNT missing value", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("zsc1"), barg("0"), barg("COUNT")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid COUNT value", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("zsc1"), barg("0"), barg("COUNT"), barg("abc")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unknown option", func(t *testing.T) {
		ctx := discardCtx("ZSCAN", [][]byte{barg("zsc1"), barg("0"), barg("BADOPT")}, s)
		if err := cmdZSCAN(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZDIFF_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZDIFF", [][]byte{barg("2")}, s)
		if err := cmdZDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid numkeys", func(t *testing.T) {
		ctx := discardCtx("ZDIFF", [][]byte{barg("abc"), barg("k1")}, s)
		if err := cmdZDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("first key not found", func(t *testing.T) {
		ctx := discardCtx("ZDIFF", [][]byte{barg("2"), barg("nosuch"), barg("k2")}, s)
		if err := cmdZDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("diff two sets", func(t *testing.T) {
		s.Set("zd1a", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		s.Set("zd1b", &store.SortedSetValue{Members: map[string]float64{"b": 2}}, store.SetOptions{})
		ctx := discardCtx("ZDIFF", [][]byte{barg("2"), barg("zd1a"), barg("zd1b")}, s)
		if err := cmdZDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WITHSCORES", func(t *testing.T) {
		ctx := discardCtx("ZDIFF", [][]byte{barg("2"), barg("zd1a"), barg("zd1b"), barg("WITHSCORES")}, s)
		if err := cmdZDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("diff yields empty", func(t *testing.T) {
		s.Set("zd2a", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		s.Set("zd2b", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		ctx := discardCtx("ZDIFF", [][]byte{barg("2"), barg("zd2a"), barg("zd2b")}, s)
		if err := cmdZDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("not enough keys", func(t *testing.T) {
		ctx := discardCtx("ZDIFF", [][]byte{barg("3"), barg("k1")}, s)
		if err := cmdZDIFF(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZUNION_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("2")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid numkeys", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("abc"), barg("k")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("numkeys < 1", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("0"), barg("k")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("union two sets", func(t *testing.T) {
		s.Set("zu1a", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2}}, store.SetOptions{})
		s.Set("zu1b", &store.SortedSetValue{Members: map[string]float64{"b": 3, "c": 4}}, store.SetOptions{})
		ctx := discardCtx("ZUNION", [][]byte{barg("2"), barg("zu1a"), barg("zu1b")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WITHSCORES", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("2"), barg("zu1a"), barg("zu1b"), barg("WITHSCORES")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WEIGHTS", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("2"), barg("zu1a"), barg("zu1b"), barg("WEIGHTS"), barg("2"), barg("3")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with AGGREGATE MIN", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("2"), barg("zu1a"), barg("zu1b"), barg("AGGREGATE"), barg("MIN")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with AGGREGATE MAX", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("2"), barg("zu1a"), barg("zu1b"), barg("AGGREGATE"), barg("MAX")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("1"), barg("nosuch")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("not enough keys", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("3"), barg("k1")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid weight", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("1"), barg("zu1a"), barg("WEIGHTS"), barg("abc")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid aggregate", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("1"), barg("zu1a"), barg("AGGREGATE"), barg("BAD")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unknown option", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("1"), barg("zu1a"), barg("BADOPT")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("AGGREGATE missing value", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("1"), barg("zu1a"), barg("AGGREGATE")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("WEIGHTS missing values", func(t *testing.T) {
		ctx := discardCtx("ZUNION", [][]byte{barg("2"), barg("zu1a"), barg("zu1b"), barg("WEIGHTS")}, s)
		if err := cmdZUNION(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZINTER_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("two sets with overlap", func(t *testing.T) {
		s.Set("zn1a", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		s.Set("zn1b", &store.SortedSetValue{Members: map[string]float64{"b": 5, "c": 6, "d": 7}}, store.SetOptions{})
		ctx := discardCtx("ZINTER", [][]byte{barg("2"), barg("zn1a"), barg("zn1b")}, s)
		if err := cmdZINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("first key not found", func(t *testing.T) {
		ctx := discardCtx("ZINTER", [][]byte{barg("1"), barg("nosuch")}, s)
		if err := cmdZINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("second key not found", func(t *testing.T) {
		s.Set("zn2", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		ctx := discardCtx("ZINTER", [][]byte{barg("2"), barg("zn2"), barg("nosuch")}, s)
		if err := cmdZINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with AGGREGATE MIN", func(t *testing.T) {
		ctx := discardCtx("ZINTER", [][]byte{barg("2"), barg("zn1a"), barg("zn1b"), barg("AGGREGATE"), barg("MIN")}, s)
		if err := cmdZINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with AGGREGATE MAX", func(t *testing.T) {
		ctx := discardCtx("ZINTER", [][]byte{barg("2"), barg("zn1a"), barg("zn1b"), barg("AGGREGATE"), barg("MAX")}, s)
		if err := cmdZINTER(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZUNIONSTORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("dst")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid numkeys", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("dst"), barg("abc"), barg("k")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("store union", func(t *testing.T) {
		s.Set("zus1a", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2}}, store.SetOptions{})
		s.Set("zus1b", &store.SortedSetValue{Members: map[string]float64{"b": 3, "c": 4}}, store.SetOptions{})
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("zusdst"), barg("2"), barg("zus1a"), barg("zus1b")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("zusdst2"), barg("1"), barg("nosuch")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with WEIGHTS", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("zusdst3"), barg("2"), barg("zus1a"), barg("zus1b"), barg("WEIGHTS"), barg("2"), barg("3")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with AGGREGATE MIN", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("zusdst4"), barg("2"), barg("zus1a"), barg("zus1b"), barg("AGGREGATE"), barg("MIN")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with AGGREGATE MAX", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("zusdst5"), barg("2"), barg("zus1a"), barg("zus1b"), barg("AGGREGATE"), barg("MAX")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("numkeys < 1", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("dst"), barg("0")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("not enough keys for numkeys", func(t *testing.T) {
		ctx := discardCtx("ZUNIONSTORE", [][]byte{barg("dst"), barg("3"), barg("k1")}, s)
		if err := cmdZUNIONSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZINTERSTORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZINTERSTORE", [][]byte{barg("dst")}, s)
		if err := cmdZINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("store intersection", func(t *testing.T) {
		s.Set("zis1a", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2}}, store.SetOptions{})
		s.Set("zis1b", &store.SortedSetValue{Members: map[string]float64{"b": 3, "c": 4}}, store.SetOptions{})
		ctx := discardCtx("ZINTERSTORE", [][]byte{barg("zisdst"), barg("2"), barg("zis1a"), barg("zis1b")}, s)
		if err := cmdZINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("first key not found", func(t *testing.T) {
		ctx := discardCtx("ZINTERSTORE", [][]byte{barg("zisdst2"), barg("1"), barg("nosuch")}, s)
		if err := cmdZINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("second key not found", func(t *testing.T) {
		s.Set("zis2", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		ctx := discardCtx("ZINTERSTORE", [][]byte{barg("zisdst3"), barg("2"), barg("zis2"), barg("nosuch")}, s)
		if err := cmdZINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty intersection result", func(t *testing.T) {
		s.Set("zis3a", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		s.Set("zis3b", &store.SortedSetValue{Members: map[string]float64{"z": 2}}, store.SetOptions{})
		ctx := discardCtx("ZINTERSTORE", [][]byte{barg("zisdst4"), barg("2"), barg("zis3a"), barg("zis3b")}, s)
		if err := cmdZINTERSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestZDIFFSTORE_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("ZDIFFSTORE", [][]byte{barg("dst"), barg("1")}, s)
		if err := cmdZDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid numkeys", func(t *testing.T) {
		ctx := discardCtx("ZDIFFSTORE", [][]byte{barg("dst"), barg("abc"), barg("k")}, s)
		if err := cmdZDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("first key not found", func(t *testing.T) {
		ctx := discardCtx("ZDIFFSTORE", [][]byte{barg("dst"), barg("1"), barg("nosuch")}, s)
		if err := cmdZDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("store diff", func(t *testing.T) {
		s.Set("zds1a", &store.SortedSetValue{Members: map[string]float64{"a": 1, "b": 2, "c": 3}}, store.SetOptions{})
		s.Set("zds1b", &store.SortedSetValue{Members: map[string]float64{"b": 2}}, store.SetOptions{})
		ctx := discardCtx("ZDIFFSTORE", [][]byte{barg("zdsdst"), barg("2"), barg("zds1a"), barg("zds1b")}, s)
		if err := cmdZDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("diff yields empty", func(t *testing.T) {
		s.Set("zds2a", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		s.Set("zds2b", &store.SortedSetValue{Members: map[string]float64{"a": 1}}, store.SetOptions{})
		ctx := discardCtx("ZDIFFSTORE", [][]byte{barg("zdsdst2"), barg("2"), barg("zds2a"), barg("zds2b")}, s)
		if err := cmdZDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("not enough keys", func(t *testing.T) {
		ctx := discardCtx("ZDIFFSTORE", [][]byte{barg("dst"), barg("3"), barg("k1")}, s)
		if err := cmdZDIFFSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

// ---------------------------------------------------------------------------
// GEO commands
// ---------------------------------------------------------------------------

func TestGEOADD_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("k"), barg("13.0"), barg("38.0")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add single member", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g1"), barg("13.361389"), barg("38.115556"), barg("Palermo")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("add multiple members", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g2"), barg("13.361389"), barg("38.115556"), barg("Palermo"), barg("15.087269"), barg("37.502669"), barg("Catania")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("NX flag", func(t *testing.T) {
		// First add a member
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		s.Set("g3", geo, store.SetOptions{})
		// NX should skip existing
		ctx := discardCtx("GEOADD", [][]byte{barg("g3"), barg("NX"), barg("0"), barg("0"), barg("Palermo"), barg("15.087269"), barg("37.502669"), barg("Catania")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("XX flag", func(t *testing.T) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		s.Set("g4", geo, store.SetOptions{})
		// XX should skip new members
		ctx := discardCtx("GEOADD", [][]byte{barg("g4"), barg("XX"), barg("0"), barg("0"), barg("Palermo"), barg("15.087269"), barg("37.502669"), barg("NewMember")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("CH flag", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g5"), barg("CH"), barg("13.361389"), barg("38.115556"), barg("Palermo")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid longitude", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g6"), barg("abc"), barg("38.0"), barg("Place")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid latitude", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g7"), barg("13.0"), barg("abc"), barg("Place")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("out of range longitude", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g8"), barg("200"), barg("38.0"), barg("Place")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("out of range latitude", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g9"), barg("13.0"), barg("90.0"), barg("Place")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		s.Set("gwt", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("GEOADD", [][]byte{barg("gwt"), barg("13.0"), barg("38.0"), barg("Place")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("remaining args wrong multiple", func(t *testing.T) {
		ctx := discardCtx("GEOADD", [][]byte{barg("g10"), barg("NX"), barg("13.0"), barg("38.0")}, s)
		if err := cmdGEOADD(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGEODIST_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEODIST", [][]byte{barg("k"), barg("m1")}, s)
		if err := cmdGEODIST(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("GEODIST", [][]byte{barg("nosuch"), barg("m1"), barg("m2")}, s)
		if err := cmdGEODIST(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("default unit (m)", func(t *testing.T) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		geo.Add("Catania", 15.087269, 37.502669)
		s.Set("gd1", geo, store.SetOptions{})
		ctx := discardCtx("GEODIST", [][]byte{barg("gd1"), barg("Palermo"), barg("Catania")}, s)
		if err := cmdGEODIST(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("km unit", func(t *testing.T) {
		ctx := discardCtx("GEODIST", [][]byte{barg("gd1"), barg("Palermo"), barg("Catania"), barg("km")}, s)
		if err := cmdGEODIST(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("mi unit", func(t *testing.T) {
		ctx := discardCtx("GEODIST", [][]byte{barg("gd1"), barg("Palermo"), barg("Catania"), barg("mi")}, s)
		if err := cmdGEODIST(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ft unit", func(t *testing.T) {
		ctx := discardCtx("GEODIST", [][]byte{barg("gd1"), barg("Palermo"), barg("Catania"), barg("ft")}, s)
		if err := cmdGEODIST(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member not found", func(t *testing.T) {
		ctx := discardCtx("GEODIST", [][]byte{barg("gd1"), barg("Palermo"), barg("NoCity")}, s)
		if err := cmdGEODIST(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGEOHASH_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEOHASH", [][]byte{barg("k")}, s)
		if err := cmdGEOHASH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("GEOHASH", [][]byte{barg("nosuch"), barg("m")}, s)
		if err := cmdGEOHASH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("existing members", func(t *testing.T) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		geo.Add("Catania", 15.087269, 37.502669)
		s.Set("gh1", geo, store.SetOptions{})
		ctx := discardCtx("GEOHASH", [][]byte{barg("gh1"), barg("Palermo"), barg("Catania")}, s)
		if err := cmdGEOHASH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member not found returns null", func(t *testing.T) {
		ctx := discardCtx("GEOHASH", [][]byte{barg("gh1"), barg("Palermo"), barg("NoCity")}, s)
		if err := cmdGEOHASH(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGEOPOS_Coverage(t *testing.T) {
	s := store.NewStore()

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEOPOS", [][]byte{barg("k")}, s)
		if err := cmdGEOPOS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("GEOPOS", [][]byte{barg("nosuch"), barg("m")}, s)
		if err := cmdGEOPOS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("existing and missing members", func(t *testing.T) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		s.Set("gp1", geo, store.SetOptions{})
		ctx := discardCtx("GEOPOS", [][]byte{barg("gp1"), barg("Palermo"), barg("NoCity")}, s)
		if err := cmdGEOPOS(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGEORADIUS_Coverage(t *testing.T) {
	s := store.NewStore()
	setupGeo := func(key string) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		geo.Add("Catania", 15.087269, 37.502669)
		geo.Add("Rome", 12.496366, 41.902782)
		s.Set(key, geo, store.SetOptions{})
	}

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("k"), barg("15"), barg("37"), barg("200")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid lon", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("k"), barg("abc"), barg("37"), barg("200"), barg("km")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid lat", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("k"), barg("15"), barg("abc"), barg("200"), barg("km")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid radius", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("k"), barg("15"), barg("37"), barg("abc"), barg("km")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("nosuch"), barg("15"), barg("37"), barg("200"), barg("km")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("basic km", func(t *testing.T) {
		setupGeo("gr1")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr1"), barg("15"), barg("37"), barg("200"), barg("km")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unit mi", func(t *testing.T) {
		setupGeo("gr2")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr2"), barg("15"), barg("37"), barg("200"), barg("mi")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unit ft", func(t *testing.T) {
		setupGeo("gr3")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr3"), barg("15"), barg("37"), barg("1000000"), barg("ft")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unit m", func(t *testing.T) {
		setupGeo("gr4")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr4"), barg("15"), barg("37"), barg("200000"), barg("m")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("WITHCOORD WITHDIST WITHHASH", func(t *testing.T) {
		setupGeo("gr5")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr5"), barg("15"), barg("37"), barg("200"), barg("km"), barg("WITHCOORD"), barg("WITHDIST"), barg("WITHHASH")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ASC sort", func(t *testing.T) {
		setupGeo("gr6")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr6"), barg("15"), barg("37"), barg("200"), barg("km"), barg("ASC")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("DESC sort", func(t *testing.T) {
		setupGeo("gr7")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr7"), barg("15"), barg("37"), barg("200"), barg("km"), barg("DESC")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("COUNT", func(t *testing.T) {
		setupGeo("gr8")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr8"), barg("15"), barg("37"), barg("200"), barg("km"), barg("COUNT"), barg("1")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("COUNT missing value", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr8"), barg("15"), barg("37"), barg("200"), barg("km"), barg("COUNT")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("COUNT invalid", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr8"), barg("15"), barg("37"), barg("200"), barg("km"), barg("COUNT"), barg("abc")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STORE", func(t *testing.T) {
		setupGeo("gr9")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr9"), barg("15"), barg("37"), barg("200"), barg("km"), barg("STORE"), barg("grdst")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STORE missing value", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr9"), barg("15"), barg("37"), barg("200"), barg("km"), barg("STORE")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST", func(t *testing.T) {
		setupGeo("gr10")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr10"), barg("15"), barg("37"), barg("200"), barg("km"), barg("STOREDIST"), barg("grddst")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST missing value", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr10"), barg("15"), barg("37"), barg("200"), barg("km"), barg("STOREDIST")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found with STORE", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("nosuch2"), barg("15"), barg("37"), barg("200"), barg("km"), barg("STORE"), barg("stout")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found with STOREDIST", func(t *testing.T) {
		ctx := discardCtx("GEORADIUS", [][]byte{barg("nosuch3"), barg("15"), barg("37"), barg("200"), barg("km"), barg("STOREDIST"), barg("sdout")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STORE empty results", func(t *testing.T) {
		setupGeo("gr11")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr11"), barg("0"), barg("0"), barg("1"), barg("km"), barg("STORE"), barg("grempty")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST empty results", func(t *testing.T) {
		setupGeo("gr12")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr12"), barg("0"), barg("0"), barg("1"), barg("km"), barg("STOREDIST"), barg("grempty2")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST with mi unit", func(t *testing.T) {
		setupGeo("gr13")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr13"), barg("15"), barg("37"), barg("200"), barg("mi"), barg("STOREDIST"), barg("grddst2")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST with ft unit", func(t *testing.T) {
		setupGeo("gr14")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr14"), barg("15"), barg("37"), barg("1000000"), barg("ft"), barg("STOREDIST"), barg("grddst3")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST with m unit", func(t *testing.T) {
		setupGeo("gr15")
		ctx := discardCtx("GEORADIUS", [][]byte{barg("gr15"), barg("15"), barg("37"), barg("200000"), barg("m"), barg("STOREDIST"), barg("grddst4")}, s)
		if err := cmdGEORADIUS(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("WITHDIST with different units", func(t *testing.T) {
		setupGeo("gr16")
		for _, unit := range []string{"m", "km", "mi", "ft"} {
			ctx := discardCtx("GEORADIUS", [][]byte{barg("gr16"), barg("15"), barg("37"), barg("200"), barg(unit), barg("WITHDIST")}, s)
			if err := cmdGEORADIUS(ctx); err != nil {
				t.Fatalf("unit %s: %v", unit, err)
			}
		}
	})
}

func TestGEORADIUSBYMEMBER_Coverage(t *testing.T) {
	s := store.NewStore()
	setupGeo := func(key string) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		geo.Add("Catania", 15.087269, 37.502669)
		s.Set(key, geo, store.SetOptions{})
	}

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("k"), barg("m"), barg("200")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid radius", func(t *testing.T) {
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("k"), barg("m"), barg("abc"), barg("km")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("nosuch"), barg("m"), barg("200"), barg("km")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("member not found", func(t *testing.T) {
		setupGeo("gbm1")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm1"), barg("NoCity"), barg("200"), barg("km")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("basic km", func(t *testing.T) {
		setupGeo("gbm2")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm2"), barg("Palermo"), barg("200"), barg("km")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unit mi", func(t *testing.T) {
		setupGeo("gbm3")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm3"), barg("Palermo"), barg("200"), barg("mi")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unit ft", func(t *testing.T) {
		setupGeo("gbm4")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm4"), barg("Palermo"), barg("1000000"), barg("ft")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("unit m", func(t *testing.T) {
		setupGeo("gbm5")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm5"), barg("Palermo"), barg("200000"), barg("m")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("WITHCOORD WITHDIST WITHHASH", func(t *testing.T) {
		setupGeo("gbm6")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm6"), barg("Palermo"), barg("200"), barg("km"), barg("WITHCOORD"), barg("WITHDIST"), barg("WITHHASH")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("ASC sort", func(t *testing.T) {
		setupGeo("gbm7")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm7"), barg("Palermo"), barg("200"), barg("km"), barg("ASC")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("DESC sort", func(t *testing.T) {
		setupGeo("gbm8")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm8"), barg("Palermo"), barg("200"), barg("km"), barg("DESC")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("COUNT", func(t *testing.T) {
		setupGeo("gbm9")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm9"), barg("Palermo"), barg("200"), barg("km"), barg("COUNT"), barg("1")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STORE", func(t *testing.T) {
		setupGeo("gbm10")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm10"), barg("Palermo"), barg("200"), barg("km"), barg("STORE"), barg("gbmdst")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST", func(t *testing.T) {
		setupGeo("gbm11")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm11"), barg("Palermo"), barg("200"), barg("km"), barg("STOREDIST"), barg("gbmddst")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found with STORE", func(t *testing.T) {
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("nosuchgbm"), barg("m"), barg("200"), barg("km"), barg("STORE"), barg("out")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STORE empty results", func(t *testing.T) {
		setupGeo("gbm12")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm12"), barg("Palermo"), barg("0.001"), barg("km"), barg("STORE"), barg("gbmempty")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST empty results", func(t *testing.T) {
		setupGeo("gbm13")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm13"), barg("Palermo"), barg("0.001"), barg("km"), barg("STOREDIST"), barg("gbmempty2")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST with mi unit", func(t *testing.T) {
		setupGeo("gbm14")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm14"), barg("Palermo"), barg("200"), barg("mi"), barg("STOREDIST"), barg("gbmddst2")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST with ft unit", func(t *testing.T) {
		setupGeo("gbm15")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm15"), barg("Palermo"), barg("1000000"), barg("ft"), barg("STOREDIST"), barg("gbmddst3")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("STOREDIST with m unit", func(t *testing.T) {
		setupGeo("gbm16")
		ctx := discardCtx("GEORADIUSBYMEMBER", [][]byte{barg("gbm16"), barg("Palermo"), barg("200000"), barg("m"), barg("STOREDIST"), barg("gbmddst4")}, s)
		if err := cmdGEORADIUSBYMEMBER(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGEOSEARCH_Coverage(t *testing.T) {
	s := store.NewStore()
	setupGeo := func(key string) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		geo.Add("Catania", 15.087269, 37.502669)
		s.Set(key, geo, store.SetOptions{})
	}

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("k"), barg("FROMLONLAT"), barg("15")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("nosuch"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMLONLAT BYRADIUS km", func(t *testing.T) {
		setupGeo("gs1")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs1"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMLONLAT invalid lon", func(t *testing.T) {
		setupGeo("gs1b")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs1b"), barg("FROMLONLAT"), barg("abc"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMMEMBER", func(t *testing.T) {
		setupGeo("gs2")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs2"), barg("FROMMEMBER"), barg("Palermo"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMMEMBER not found", func(t *testing.T) {
		setupGeo("gs3")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs3"), barg("FROMMEMBER"), barg("NoCity"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS mi", func(t *testing.T) {
		setupGeo("gs4")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs4"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("mi")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS ft", func(t *testing.T) {
		setupGeo("gs5")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs5"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("1000000"), barg("ft")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS m", func(t *testing.T) {
		setupGeo("gs6")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs6"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200000"), barg("m")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYBOX", func(t *testing.T) {
		setupGeo("gs7")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs7"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYBOX"), barg("400"), barg("400"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS invalid radius", func(t *testing.T) {
		setupGeo("gs8")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs8"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("abc"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYBOX invalid width", func(t *testing.T) {
		setupGeo("gs9")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs9"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYBOX"), barg("abc"), barg("400"), barg("km")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMMEMBER missing value", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs1"), barg("FROMMEMBER")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMLONLAT missing values", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs1"), barg("FROMLONLAT"), barg("15")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS missing values", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs1"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYBOX missing values", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs1"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYBOX"), barg("400")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with ASC and COUNT options", func(t *testing.T) {
		setupGeo("gs10")
		ctx := discardCtx("GEOSEARCH", [][]byte{barg("gs10"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km"), barg("ASC"), barg("COUNT"), barg("WITHDIST")}, s)
		if err := cmdGEOSEARCH(ctx); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGEOSEARCHSTORE_Coverage(t *testing.T) {
	s := store.NewStore()
	setupGeo := func(key string) {
		geo := store.NewGeoValue()
		geo.Add("Palermo", 13.361389, 38.115556)
		geo.Add("Catania", 15.087269, 37.502669)
		s.Set(key, geo, store.SetOptions{})
	}

	t.Run("too few args", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("dst"), barg("src"), barg("FROMLONLAT"), barg("15")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("src not found", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("dst"), barg("nosuch"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMLONLAT BYRADIUS", func(t *testing.T) {
		setupGeo("gss1")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst1"), barg("gss1"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMMEMBER", func(t *testing.T) {
		setupGeo("gss2")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst2"), barg("gss2"), barg("FROMMEMBER"), barg("Palermo"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMMEMBER not found", func(t *testing.T) {
		setupGeo("gss3")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst3"), barg("gss3"), barg("FROMMEMBER"), barg("NoCity"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMLONLAT invalid", func(t *testing.T) {
		setupGeo("gss4")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst4"), barg("gss4"), barg("FROMLONLAT"), barg("abc"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS invalid", func(t *testing.T) {
		setupGeo("gss5")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst5"), barg("gss5"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("abc"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYBOX", func(t *testing.T) {
		setupGeo("gss6")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst6"), barg("gss6"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYBOX"), barg("400"), barg("400"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("empty results deletes dest", func(t *testing.T) {
		setupGeo("gss7")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst7"), barg("gss7"), barg("FROMLONLAT"), barg("0"), barg("0"), barg("BYRADIUS"), barg("1"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS mi", func(t *testing.T) {
		setupGeo("gss8")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst8"), barg("gss8"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("mi")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS ft", func(t *testing.T) {
		setupGeo("gss9")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst9"), barg("gss9"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("1000000"), barg("ft")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS m", func(t *testing.T) {
		setupGeo("gss10")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst10"), barg("gss10"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200000"), barg("m")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMMEMBER missing value", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("dst"), barg("gss1"), barg("FROMMEMBER")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("FROMLONLAT missing values", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("dst"), barg("gss1"), barg("FROMLONLAT"), barg("15")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYRADIUS missing values", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("dst"), barg("gss1"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("BYBOX missing values", func(t *testing.T) {
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("dst"), barg("gss1"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYBOX"), barg("400")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with BYBOX invalid width", func(t *testing.T) {
		setupGeo("gss11")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst11"), barg("gss11"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYBOX"), barg("abc"), barg("400"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("with ASC DESC STOREDIST options", func(t *testing.T) {
		setupGeo("gss12")
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst12"), barg("gss12"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km"), barg("ASC"), barg("STOREDIST")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wrong type dest", func(t *testing.T) {
		setupGeo("gss13")
		s.Set("gssdst13", &store.StringValue{Data: []byte("x")}, store.SetOptions{})
		ctx := discardCtx("GEOSEARCHSTORE", [][]byte{barg("gssdst13"), barg("gss13"), barg("FROMLONLAT"), barg("15"), barg("37"), barg("BYRADIUS"), barg("200"), barg("km")}, s)
		if err := cmdGEOSEARCHSTORE(ctx); err != nil {
			t.Fatal(err)
		}
	})
}
