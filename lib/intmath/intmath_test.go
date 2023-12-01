package intmath

import (
	"math"
	"testing"
)

const sqrt32Input = 100

var sqrtFns = []struct {
	name string
	fn   func(int32) int32
}{
	{"f64", sqrt32_f64},
	{"fsqrt", sqrt32_fsqrt},
	{"julery", sqrt32_julery},
	{"3dcoder", sqrt32_3dcoder},
}

var sqrtInputs = []struct {
	name  string
	input int32
}{
	{"small", 100},
	{"medium", 10000},
	{"large", 1000000},
	{"huge", int32(math.Pow(math.MaxInt16-1, 2))},
}

func BenchmarkSqrt32(b *testing.B) {
	for _, sqrtFn := range sqrtFns {
		for _, sqrtInput := range sqrtInputs {
			b.Run(sqrtFn.name+"_"+sqrtInput.name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					sqrtFn.fn(sqrtInput.input)
				}
			})
		}
	}
}

func TestSqrt32(t *testing.T) {
	t.Logf("using sqrt32_%s as baseline", sqrtFns[0].name)

	for _, sqrtInput := range sqrtInputs {
		baseline := sqrtFns[0].fn(sqrtInput.input)
		for _, sqrtFn := range sqrtFns[1:] {
			r := sqrtFn.fn(sqrtInput.input)
			if r != baseline {
				t.Errorf(
					"sqrt32_%s(%d) = %d, want %d",
					sqrtFn.name, sqrtInput.input, r, baseline)
			}
		}
	}
}
