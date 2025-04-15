package metricring

import (
	"sync"
)

type MetricInfo struct {
	Name  string
	Value float64
}

type MetricSnapshot struct {
	Info MetricInfo
	Time int64
}

type MetricRing struct {
	MaxLen       int
	Queue        []*MetricSnapshot
	CurrentIndex int

	sync.RWMutex
}

func (ring *MetricRing) Sum() float64 {
	ring.RLock()
	defer ring.RUnlock()

	sum := 0.0
	for _, snapshot := range ring.Queue {
		if snapshot != nil {
			sum += snapshot.Info.Value
		}
	}
	return sum
}

func (ring *MetricRing) Push(snapShot *MetricSnapshot) {
	ring.Lock()
	defer ring.Unlock()

	if ring.CurrentIndex != -1 && snapShot != nil {
		latestSnapShot := ring.Queue[ring.CurrentIndex]
		if latestSnapShot != nil && latestSnapShot.Time == snapShot.Time {
			return
		}
	}

	ring.CurrentIndex = (ring.CurrentIndex + 1) % ring.MaxLen
	ring.Queue[ring.CurrentIndex] = snapShot
}

func (ring *MetricRing) OverCount(threshold float64) (overCount int) {
	ring.RLock()
	defer ring.RUnlock()

	for _, snapshot := range ring.Queue {
		if snapshot == nil {
			continue
		}

		if snapshot.Info.Value > threshold {
			overCount++
		}
	}
	return
}

func CreateMetricRing(size int) *MetricRing {
	return &MetricRing{
		MaxLen:       size,
		Queue:        make([]*MetricSnapshot, size),
		CurrentIndex: -1,
	}
}
