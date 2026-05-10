package timestamps

import (
	"math/rand"
	"time"
)

func GenerateRandom(duration float64, count int) []float64 {
	if count <= 0 {
		return nil
	}
	if duration <= 0 {
		return []float64{0}
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	seconds := make([]float64, count)
	
	minInterval := duration / float64(count+1)
	
	for i := 0; i < count; i++ {
		baseTime := minInterval * float64(i+1)
		jitter := rng.Float64() * minInterval * 0.8
		seconds[i] = baseTime + jitter
	}
	
	return seconds
}