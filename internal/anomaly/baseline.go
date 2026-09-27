package anomaly

import "math"

// baseline tracks a running mean and sample std-dev of RSSI via Welford's algorithm.
type baseline struct {
	n    int
	mean float64
	m2   float64
}

func (b *baseline) add(x float64) {
	b.n++
	d := x - b.mean
	b.mean += d / float64(b.n)
	b.m2 += d * (x - b.mean)
}

func (b *baseline) std() float64 {
	if b.n < 2 {
		return 0
	}
	return math.Sqrt(b.m2 / float64(b.n-1))
}

// z returns the z-score of x against the current baseline (0 if std is 0).
func (b *baseline) z(x float64) float64 {
	s := b.std()
	if s == 0 {
		return 0
	}
	return (x - b.mean) / s
}
