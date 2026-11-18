package p8compress

import (
	"os"
	"testing"
)

func BenchmarkCompress64K(b *testing.B) {
	code, err := os.ReadFile("../../testdata/p8compress/near_limit.lua")
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(code)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := Compress(code); !ok {
			b.Fatal("unavailable")
		}
	}
}
