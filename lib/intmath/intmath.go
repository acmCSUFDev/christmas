package intmath

import (
	"math"

	"golang.org/x/exp/constraints"
)

func Min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Abs[T constraints.Signed](a T) T {
	if a < 0 {
		return -a
	}
	return a
}

// Sqrt32 returns the square root of x, truncated to an int32.
func Sqrt32(x int32) int32 {
	return sqrt32_f64(x)
}

func sqrt32_f64(x int32) int32 {
	return int32(math.Sqrt(float64(x)))
}

// https://stackoverflow.com/a/31120562/5041327
func sqrt32_julery(x int32) int32 {
	var temp, g int32
	var b int32 = 0x8000
	var bshft int32 = 15

	for {
		temp = (((g << 1) + b) << bshft)
		if int32(x) >= temp {
			g += b
			x -= int32(temp)
		}
		b >>= 1
		bshft--
		if b == 0 {
			break
		}
	}

	return g
}

// https://stackoverflow.com/q/31117497/5041327
func sqrt32_3dcoder(x int32) int32 {
	var res int32
	var add int32 = 0x8000

	for i := 0; i < 16; i++ {
		temp := res | add
		g2 := temp * temp
		if x >= g2 {
			res = temp
		}
		add >>= 1
	}

	return res
}

// https://stackoverflow.com/a/1101217/5041327
func sqrt32_fsqrt(x int32) int32 {
	var res int32
	one := int32(1) << 30

	for one > x {
		one >>= 2
	}

	for one != 0 {
		if x >= res+one {
			x = x - (res + one)
			res = res + 2*one
		}
		res >>= 1
		one >>= 2
	}

	if x > res {
		res++ // correct for rounding error
	}

	return res
}
