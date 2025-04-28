package strategy

import (
	"context"
	"github.com/kubewharf/katalyst-core/pkg/config/agent/dynamic/metricthreshold"
	"github.com/kubewharf/katalyst-core/pkg/consts"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric/types"
	"github.com/kubewharf/katalyst-core/pkg/util/general"
	"github.com/kubewharf/katalyst-core/pkg/util/strategygroup"
	"strconv"
)

func (p *NumaCPUPressureEviction) pullThresholds(_ context.Context) {
	enabled, err := strategygroup.IsStrategyEnabledForNode(consts.StrategyNameNumaCpuPressureEviction, false, p.conf)
	if err != nil {
		general.Errorf("failed to get eviction strategy: %v", err)
		return
	}
	p.enabled = enabled
	if !enabled {
		general.Warningf("eviction strategy is disabled, skip pullThresholds")
		return
	}
	general.Infof("%v update enabled", EvictionNameNumaCpuPressure)

	cpuCodeName, isVM := getCpuCodeAndIsVm(p.metaServer.MetricsFetcher)

	globalThresholds := p.conf.DynamicAgentConfiguration.GetDynamicConfiguration().MetricThreshold
	thresholds := getOverLoadThreshold(globalThresholds, cpuCodeName, isVM)
	thresholds = convertThreshold(thresholds)
	expandedThresholds := expandThresholds(thresholds, p.numaPressureConfig.ExpandFactor)

	// update
	p.Lock()
	defer p.Unlock()
	general.Infof("update thresholds to %v", expandedThresholds)
	p.thresholds = expandedThresholds
}

func expandThresholds(thresholds map[string]float64, expandFactor float64) map[string]float64 {
	for key, val := range thresholds {
		thresholds[key] = val * expandFactor
	}
	return thresholds
}

func getCpuCodeAndIsVm(metricsFetcher types.MetricsFetcher) (string, bool) {
	cpuCodeNameInterface := metricsFetcher.GetByStringIndex(consts.MetricCPUCodeName)
	cpuCodeName, ok := cpuCodeNameInterface.(string)
	if !ok {
		general.Warningf("parse cpu code name %v failed", cpuCodeNameInterface)
		cpuCodeName = ""
	}
	// todo may not be set when get...
	isVM := false
	isVMInterface := metricsFetcher.GetByStringIndex(consts.MetricInfoIsVM)
	isVMStr, ok := isVMInterface.(string)
	if !ok {
		general.Warningf("parse is vm %v failed", isVMInterface)
	} else {
		parseBool, err := strconv.ParseBool(isVMStr)
		if err != nil {
			general.Warningf("parse is vm %v failed %v", isVMInterface, err)
		} else {
			isVM = parseBool
		}
	}
	return cpuCodeName, isVM
}

func getOverLoadThreshold(globalThresholds *metricthreshold.MetricThreshold, cpuCode string, isVM bool) map[string]float64 {
	modelThresholds, exists := globalThresholds.Threshold[cpuCode]
	if !exists {
		general.Warningf("no suitable threshold for cpuCode %v using default", isVM)
		modelThresholds = globalThresholds.Threshold[metricthreshold.DefaultCPUCodeName]
	}
	threshold, exists := modelThresholds[isVM]
	if !exists {
		general.Warningf("no suitable threshold for cpuCode %v isVm %v, using default", cpuCode, isVM)
		return modelThresholds[false]
	}
	return threshold
}

func convertThreshold(origin map[string]float64) map[string]float64 {
	res := map[string]float64{}
	for k, v := range origin {
		if k == "cpu_usage_threshold" {
			res[consts.MetricCPUUsageContainer] = v
		} else if k == "cpu_load_threshold" {
			res[consts.MetricLoad1MinContainer] = v
		}
	}
	return res
}
