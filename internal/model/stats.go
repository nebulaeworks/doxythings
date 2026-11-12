package model

import (
	"math"
	"slices"
)

/**
 * Stats summarises one quantity across a run's repetitions, in nanoseconds.
 *
 * Every field but N is NaN when there were no samples.
 */
type Stats struct {
	/** N is the number of samples. */
	N int
	/** Mean is the arithmetic mean. */
	Mean float64
	/** Median is the middle sample, or the mean of the two middle samples. */
	Median float64
	/** StdDev is the sample standard deviation; 0 for a single sample. */
	StdDev float64
	/** Min is the smallest sample. */
	Min float64
	/** Max is the largest sample. */
	Max float64
}

/**
 * StatsOf summarises values.
 *
 * @param values The samples, in any order.
 * @return Their statistics; empty for no samples.
 */
func StatsOf(values []float64) Stats {
	if len(values) == 0 {
		nan := math.NaN()
		return Stats{0, nan, nan, nan, nan, nan}
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	mean := exactSum(values) / float64(len(values))
	return Stats{
		N:      len(values),
		Mean:   mean,
		Median: median(sorted),
		StdDev: sampleStdDev(values, mean),
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
	}
}

/**
 * IsEmpty reports whether the summary has no samples.
 *
 * @return Whether N is 0.
 */
func (s Stats) IsEmpty() bool {
	return s.N == 0 || math.IsNaN(s.Mean)
}

/**
 * median returns the median of an already sorted, non-empty slice.
 *
 * @param sorted The samples, sorted.
 * @return Their median.
 */
func median(sorted []float64) float64 {
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

/**
 * sampleStdDev returns the sample (n-1) standard deviation of values around
 * mean, or 0 for a single value.
 *
 * @param values The samples.
 * @param mean Their mean.
 * @return The standard deviation.
 */
func sampleStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	squares := make([]float64, len(values))
	for i, v := range values {
		squares[i] = (v - mean) * (v - mean)
	}
	return math.Sqrt(exactSum(squares) / float64(len(values)-1))
}

/**
 * exactSum adds values without accumulating rounding error (Shewchuk's
 * algorithm), so means don't depend on the order samples arrived in.
 *
 * @param values The numbers to add.
 * @return Their sum, correctly rounded.
 */
func exactSum(values []float64) float64 {
	var partials []float64
	for _, x := range values {
		kept := partials[:0]
		for _, y := range partials {
			if math.Abs(x) < math.Abs(y) {
				x, y = y, x
			}
			hi := x + y
			lo := y - (hi - x)
			if lo != 0 {
				kept = append(kept, lo)
			}
			x = hi
		}
		partials = append(kept, x)
	}
	total := 0.0
	for _, p := range partials {
		total += p
	}
	return total
}
