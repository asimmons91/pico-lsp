package p8png

import (
	"os"
	"testing"
)

func FuzzDecode(f *testing.F) {
	if data, err := os.ReadFile("../../testdata/p8png/smoke.p8.png"); err == nil {
		f.Add(data)
	}
	f.Add([]byte("not a png"))
	f.Add([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := Decode(data)
		if err != nil {
			return
		}
		if result.Text == "" || result.Code == "" {
			t.Fatal("decoded empty result")
		}
	})
}
