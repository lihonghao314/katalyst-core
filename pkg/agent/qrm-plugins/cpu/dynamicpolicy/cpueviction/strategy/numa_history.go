package strategy

import (
	"time"
)

const (
	FakePodUID = ""
)

type NumaMetricHistory struct {
	// numa -> pod -> metric -> ring
	Inner    map[int]map[string]map[string]*MetricRing
	RingSize int
}

func NewMetricHistory(ringSize int) *NumaMetricHistory {
	return &NumaMetricHistory{
		Inner:    make(map[int]map[string]map[string]*MetricRing),
		RingSize: ringSize,
	}
}

func (m *NumaMetricHistory) Push(numaID int, podUID string, metricName string, podMetric float64) {
	collectTime := time.Now().UnixNano()

	if m.Inner[numaID] == nil {
		m.Inner[numaID] = make(map[string]map[string]*MetricRing)
	}
	if m.Inner[numaID][podUID] == nil {
		m.Inner[numaID][podUID] = make(map[string]*MetricRing)
	}
	if m.Inner[numaID][podUID][metricName] == nil {
		m.Inner[numaID][podUID][metricName] = CreateMetricRing(m.RingSize)
	}
	snapshot := &MetricSnapshot{
		Info: MetricInfo{
			Name:  metricName,
			Value: podMetric,
		},
		Time: collectTime,
	}
	m.Inner[numaID][podUID][metricName].Push(snapshot)
}

func (m *NumaMetricHistory) PushNuma(numaID int, metricName string, podMetric float64) {
	m.Push(numaID, FakePodUID, metricName, podMetric)
}
