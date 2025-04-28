package eviction

import (
	"github.com/kubewharf/katalyst-core/pkg/config/agent/dynamic/crd"
)

type NumaCPUPressureEvictionConfiguration struct {
	NumaCpuPressureEnableEviction         bool
	NumaCpuPressureThresholdMetPercentage float64
	NumaCpuPressureMetricRingSize         int
	NumaCpuPressureGracePeriod            int64
	NumaCpuThresholdExpandFactor          float64
}

func NewNumaCPUPressureEvictionConfiguration() *NumaCPUPressureEvictionConfiguration {
	return &NumaCPUPressureEvictionConfiguration{
		NumaCpuPressureEnableEviction:         false,
		NumaCpuPressureThresholdMetPercentage: 0.7,
		NumaCpuPressureMetricRingSize:         4,
		NumaCpuPressureGracePeriod:            60,
		NumaCpuThresholdExpandFactor:          1.1,
	}
}

func (n *NumaCPUPressureEvictionConfiguration) ApplyConfiguration(conf *crd.DynamicConfigCRD) {

}
