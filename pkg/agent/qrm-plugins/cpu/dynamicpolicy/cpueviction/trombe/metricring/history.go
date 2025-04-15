package metricring

import (
	"time"
)

const (
	FakePodUID = ""
)

type MetricHistory struct {
	Inner    map[int]map[string]map[string]*MetricRing
	ringSize int
}

func NewMetricHistory(ringSize int) *MetricHistory {
	return &MetricHistory{
		Inner:    make(map[int]map[string]map[string]*MetricRing),
		ringSize: ringSize,
	}
}

func (m *MetricHistory) Push(numaID int, podUID string, metricName string, podMetric float64) {
	collectTime := time.Now().UnixNano()

	if m.Inner[numaID] == nil {
		m.Inner[numaID] = make(map[string]map[string]*MetricRing)
	}
	if m.Inner[numaID][podUID] == nil {
		m.Inner[numaID][podUID] = make(map[string]*MetricRing)
	}
	if m.Inner[numaID][podUID][metricName] == nil {
		m.Inner[numaID][podUID][metricName] = CreateMetricRing(m.ringSize)
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

func (m *MetricHistory) PushNuma(numaID int, metricName string, podMetric float64) {
	m.Push(numaID, FakePodUID, metricName, podMetric)
}
