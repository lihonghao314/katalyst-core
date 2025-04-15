package trombe

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/kubewharf/katalyst-core/pkg/config"
	"github.com/kubewharf/katalyst-core/pkg/consts"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric/types"
	"github.com/kubewharf/katalyst-core/pkg/util/general"
	"github.com/kubewharf/katalyst-core/pkg/util/strategygroup"
)

var (
	defaultCPUCodeName = "default"
)

type GlobalThresholds map[string]map[string]map[string]float64

// todo refactor
func (p *NumaCPUPressureEviction) pullThresholds(_ context.Context) {
	//strategy, err := getEvictionStrategy(p.conf)
	//strategy, err := mockEvictionStrategy(p.conf)
	//if err != nil {
	//	general.Errorf("Failed to get eviction strategy: %v", err)
	//	return
	//}
	enabled, err := strategygroup.IsStrategyEnabledForNode(consts.StrategyNameNumaCpuPressureEviction, false, p.conf)
	if err != nil {
		general.Errorf("failed to get eviction strategy: %v", err)
		return
	}
	general.Infof("%v update enabled", EvictionNameNumaCpuPressure)
	p.enabled = enabled
	if !enabled {
		general.Warningf("eviction strategy is disabled, skip pullThresholds")
		return
	}

	globalThresholds := mockThresholds()
	cpuCodeName, isVM := getCpuCodeAndIsVm(p.metaServer.MetricsFetcher)
	thresholds := getOverLoadThreshold(globalThresholds, cpuCodeName, isVM)
	thresholds = expandThresholds(thresholds, p.loadConfig.ExpandFactor)

	// update
	p.Lock()
	defer p.Unlock()
	general.Infof("update thresholds %v", thresholds)
	p.thresholds = thresholds
}
func expandThresholds(thresholds map[string]float64, expandFactor float64) map[string]float64 {
	for key, val := range thresholds {
		thresholds[key] = val * expandFactor
	}
	return thresholds
}

func getCpuCodeAndIsVm(metricsFetcher types.MetricsFetcher) (string, string) {
	cpuCodeNameInterface := metricsFetcher.GetByStringIndex(consts.MetricCPUCodeName)
	cpuCodeName, ok := cpuCodeNameInterface.(string)
	if !ok {
		general.Warningf("parse cpu code name %v failed", cpuCodeNameInterface)
		cpuCodeName = ""
	}
	// todo may not be set when get...
	isVMInterface := metricsFetcher.GetByStringIndex(consts.MetricInfoIsVM)
	isVM, ok := isVMInterface.(string)
	if !ok {
		general.Warningf("parse is vm %v failed", isVMInterface)
	}
	return cpuCodeName, isVM
}

// todo get strategy from sg or aqc or new cr
func mockThresholds() map[string]map[string]map[string]float64 {
	return map[string]map[string]map[string]float64{
		"Intel_CascadeLake": {
			"false": {
				"cpu.usage.numa.container": 0.55,
				"cpu.load.1min.container":  0.68,
			},
			"true": {
				"cpu.usage.numa.container": 0.54,
				"cpu.load.1min.container":  0.68,
			},
		},
		"Intel_EmeraldRapids": {
			"false": {
				"cpu.usage.numa.container": 0.6,
				"cpu.load.1min.container":  1.0,
			},
			"true": {
				"cpu.usage.numa.container": 0.6,
				"cpu.load.1min.container":  0.75,
			},
		},
		"AMD_K19Zen4": {
			"false": {
				"cpu.usage.numa.container": 0.55,
				"cpu.load.1min.container":  0.7,
			},
			"true": {
				"cpu.usage.numa.container": 0.55,
				"cpu.load.1min.container":  0.7,
			},
		},
		"Intel_SapphireRapids": {
			"false": {
				"cpu.usage.numa.container": 0.59,
				"cpu.load.1min.container":  1.0,
			},
			"true": {
				"cpu.usage.numa.container": 0.59,
				"cpu.load.1min.container":  1.0,
			},
		},
		"AMD_K19Zen3": {
			"false": {
				"cpu.usage.numa.container": 0.55,
				"cpu.load.1min.container":  0.7,
			},
			"true": {
				"cpu.usage.numa.container": 0.55,
				"cpu.load.1min.container":  0.6,
			},
		},
		"Intel_IceLake": {
			"false": {
				"cpu.usage.numa.container": 0.57,
				"cpu.load.1min.container":  0.7,
			},
			"true": {
				"cpu.usage.numa.container": 0.54,
				"cpu.load.1min.container":  0.7,
			},
		},
		"AMD_K17Zen2": {
			"false": {
				"cpu.usage.numa.container": 0.51,
				"cpu.load.1min.container":  0.68,
			},
		},
		"Intel_SkyLake": {
			"false": {
				"cpu.usage.numa.container": 0.53,
				"cpu.load.1min.container":  0.68,
			},
			"true": {
				"cpu.usage.numa.container": 0.53,
				"cpu.load.1min.container":  0.68,
			},
		},
		"Intel_Broadwell": {
			"false": {
				"cpu.usage.numa.container": 0.47,
				"cpu.load.1min.container":  0.68,
			},
		},
		defaultCPUCodeName: {
			"false": {
				"cpu.usage.numa.container": 0.6,
				"cpu.load.1min.container":  1.0,
			},
			"true": {
				"cpu.usage.numa.container": 0.6,
				"cpu.load.1min.container":  1.0,
			},
		},
	}
}

func getEvictionStrategy(conf *config.Configuration) (*EvictionStrategy, error) {
	// get strategy
	strategyContent, enabled, err := strategygroup.GetSpecificStrategyParam(consts.StrategyNameNumaCpuPressureEviction, conf)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, fmt.Errorf("eviction strategy not enabled")
	}
	// unmarshall
	strategy, err := parseStrategy(strategyContent)
	if err != nil {
		return nil, err
	}
	return &strategy, nil
}

func parseStrategy(strategyParam string) (EvictionStrategy, error) {
	var ret EvictionStrategy
	err := json.Unmarshal([]byte(strategyParam), &ret)
	if err != nil {
		return EvictionStrategy{}, err
	}
	return ret, nil
}

type EvictionStrategy struct {
	// cpu_code -> is_vm -> metric -> threshold
	OverloadThresholds map[string]map[string]map[string]float64
}

func getOverLoadThreshold(thresholds GlobalThresholds, cpuCode string, isVM string) map[string]float64 {
	modelThresholds, exists := thresholds[cpuCode]
	if !exists {
		general.Warningf("no suitable threshold for cpuCode %s using default", isVM)
		modelThresholds = thresholds[defaultCPUCodeName]
	}
	threshold, exists := modelThresholds[isVM]
	if !exists {
		general.Warningf("no suitable threshold for cpuCode %s isVm %s, using default", cpuCode, isVM)
		return modelThresholds["false"]
	}
	return threshold
}
