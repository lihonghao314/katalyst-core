package trombe

import (
	"context"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"fmt"
	pluginapi "github.com/kubewharf/katalyst-api/pkg/protocol/evictionplugin/v1alpha1"
	"github.com/kubewharf/katalyst-core/pkg/agent/qrm-plugins/cpu/dynamicpolicy/cpueviction/strategy"
	"github.com/kubewharf/katalyst-core/pkg/agent/qrm-plugins/cpu/dynamicpolicy/cpueviction/trombe/metricring"
	"github.com/kubewharf/katalyst-core/pkg/agent/qrm-plugins/cpu/dynamicpolicy/state"
	"github.com/kubewharf/katalyst-core/pkg/config"
	"github.com/kubewharf/katalyst-core/pkg/consts"
	"github.com/kubewharf/katalyst-core/pkg/metaserver"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric/helper"
	"github.com/kubewharf/katalyst-core/pkg/metrics"
	"github.com/kubewharf/katalyst-core/pkg/util/general"
	"github.com/kubewharf/katalyst-core/pkg/util/metric"
	"sort"
	"strconv"
)

const EvictionNameNumaCpuPressure = "numa-cpu-pressure-plugin"

const evictionConditionCPUUsagePressure = "NumaCPUPressure"

const (
	metricsNameCollectMetricsCalled = "numa_cpu_pressure_usage_collect_metrics_called"
	//metricsNamePodRaw  = "numa_cpu_pressure_pod_raw"
	metricsNameNumaRaw = "numa_cpu_pressure_numa_raw"

	metricsNameThresholdMet = "numa_cpu_pressure_threshold_met"
	metricsNameGetEvictPods = "numa_cpu_pressure_get_evict_pods"

	metricNameNumaOverloadNumaCount = "numa_cpu_pressure_overload_numa_count"
	metricNameNumaOverloadRatio     = "numa_cpu_pressure_overload_ratio"
	metricNameNumaOverloadTopPod    = "numa_cpu_pressure_overload_top_pod"

	metricNameMetricNoThreshold = "numa_cpu_pressure_metric_no_threshold"

	metricTagMetricName = "metric_name"
	metricTagNuma       = "numa"
	//metricTagPod        = "pod"
	metricTagIsOverload = "is_overload"
)

var (
	targetMetric = consts.MetricsCPUUsageNUMAContainer
	//targetMetrics      = []string{consts.MetricCPUUsageContainer, consts.MetricLoad1MinContainer}
	targetMetrics      = []string{consts.MetricsCPUUsageNUMAContainer}
	filterPodContainer = func(pod *v1.Pod, container *v1.Container) bool {
		return true
	}
)

type NumaCPUPressureEviction struct {
	sync.Mutex
	emitter    metrics.MetricEmitter
	metaServer *metaserver.MetaServer

	conf       *config.Configuration
	loadConfig *LoadConfig

	syncPeriod time.Duration

	thresholds map[string]float64

	metricsHistory    *metricring.MetricHistory
	overloadNumaCount int

	enabled bool
}

func NewCPUPressureUsageEviction(emitter metrics.MetricEmitter, metaServer *metaserver.MetaServer,
	conf *config.Configuration, _ state.ReadonlyState) (strategy.CPUPressureEviction, error) {

	loadConfig := &LoadConfig{
		MetricRingSize:         4,
		ThresholdMetPercentage: 0.7,
		GracePeriod:            conf.DynamicAgentConfiguration.GetDynamicConfiguration().DeletionGracePeriod,
		ExpandFactor:           1.1,
	}

	return &NumaCPUPressureEviction{
		emitter:        emitter,
		metaServer:     metaServer,
		conf:           conf,
		loadConfig:     loadConfig,
		metricsHistory: metricring.NewMetricHistory(loadConfig.MetricRingSize),
		syncPeriod:     15 * time.Second,
	}, nil
}

func (p *NumaCPUPressureEviction) Start(ctx context.Context) (err error) {
	general.Infof("%s", p.Name())

	go wait.UntilWithContext(ctx, p.update, p.syncPeriod)
	go wait.UntilWithContext(ctx, p.pullThresholds, p.syncPeriod)
	return
}

func (p *NumaCPUPressureEviction) Name() string { return EvictionNameNumaCpuPressure }

func (p *NumaCPUPressureEviction) GetEvictPods(_ context.Context, request *pluginapi.GetEvictPodsRequest) (*pluginapi.GetEvictPodsResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("GetTopEvictionPods got nil request")
	} else if len(request.ActivePods) == 0 {
		general.Warningf("got empty active pods list")
		return &pluginapi.GetEvictPodsResponse{}, nil
	}

	if !p.enabled {
		general.Infof("numa cpu pressure eviction is disabled")
		return &pluginapi.GetEvictPodsResponse{}, nil
	}

	//enabled, err := strategygroup.IsStrategyEnabledForNode(consts.StrategyNameNumaCpuPressureEviction, false, p.conf)
	//if err != nil {
	//	general.Errorf("failed to get eviction strategy: %v", err)
	//	return &pluginapi.GetTopEvictionPodsResponse{}, nil
	//
	//}
	//if !enabled {
	//	general.Warningf("eviction strategy is disabled")
	//	return &pluginapi.GetTopEvictionPodsResponse{}, nil
	//}

	p.Lock()
	defer p.Unlock()

	if p.overloadNumaCount == 0 {
		_ = p.emitter.StoreInt64(metricsNameGetEvictPods, 0, metrics.MetricTypeNameRaw)
		return &pluginapi.GetEvictPodsResponse{}, nil
	}

	numaID, numaOverloadRatio, err := p.pickTopOverRatioNuma(targetMetric, p.thresholds)
	if err != nil {
		general.ErrorS(err, "pick top over ratio numa failed")
		_ = p.emitter.StoreFloat64(metricsNameGetEvictPods, 0, metrics.MetricTypeNameRaw,
			metrics.ConvertMapToTags(map[string]string{
				metricTagMetricName: targetMetric,
				metricTagNuma:       strconv.Itoa(numaID),
			})...)
		return &pluginapi.GetEvictPodsResponse{}, nil
	}

	topPod, podOverloadRatio, err := p.pickTopOverRatioPod(numaID, targetMetric, p.thresholds, request.ActivePods)
	if err != nil {
		general.ErrorS(err, "pick top over ratio nums pods failed")
		_ = p.emitter.StoreFloat64(metricsNameGetEvictPods, 0, metrics.MetricTypeNameRaw,
			metrics.ConvertMapToTags(map[string]string{
				metricTagMetricName: targetMetric,
				metricTagNuma:       strconv.Itoa(numaID),
			})...)
		return &pluginapi.GetEvictPodsResponse{}, nil
	}

	general.InfoS("evict pod", "pod", topPod.Name, "podOverloadRatio", podOverloadRatio,
		"numaOverloadRatio", numaOverloadRatio)

	resp := &pluginapi.GetEvictPodsResponse{
		EvictPods: []*pluginapi.EvictPod{
			{
				Pod: topPod,
				Reason: fmt.Sprintf("numa cpu usage %f overload, kill top pod with %f",
					numaOverloadRatio, podOverloadRatio),
				ForceEvict:         true,
				EvictionPluginName: EvictionNameNumaCpuPressure,
			},
		},
	}

	if gracePeriod := p.loadConfig.GracePeriod; gracePeriod >= 0 {
		deletionOptions := &pluginapi.DeletionOptions{
			GracePeriodSeconds: gracePeriod,
		}
		for _, e := range resp.EvictPods {
			e.DeletionOptions = deletionOptions
		}
	}

	_ = p.emitter.StoreFloat64(metricsNameGetEvictPods, 1, metrics.MetricTypeNameRaw,
		metrics.ConvertMapToTags(map[string]string{
			metricTagMetricName: targetMetric,
			metricTagNuma:       strconv.Itoa(numaID),
		})...)

	return resp, nil
}

func (p *NumaCPUPressureEviction) ThresholdMet(_ context.Context, _ *pluginapi.Empty,
) (*pluginapi.ThresholdMetResponse, error) {
	//enabled, err := strategygroup.IsStrategyEnabledForNode(consts.StrategyNameNumaCpuPressureEviction, false, p.conf)
	//if err != nil {
	//	general.Errorf("failed to get eviction strategy: %v", err)
	//	return &pluginapi.ThresholdMetResponse{MetType: pluginapi.ThresholdMetType_NOT_MET}, nil
	//}
	//if !enabled {
	//	general.Warningf("eviction strategy is disabled")
	//	return &pluginapi.ThresholdMetResponse{MetType: pluginapi.ThresholdMetType_NOT_MET}, nil
	//}
	if !p.enabled {
		general.Infof("numa cpu pressure eviction is disabled")
		return &pluginapi.ThresholdMetResponse{
			MetType: pluginapi.ThresholdMetType_NOT_MET,
		}, nil
	}

	p.Lock()
	defer p.Unlock()

	nodeOverload := p.isNodeOverload()

	if !nodeOverload {
		_ = p.emitter.StoreFloat64(metricsNameThresholdMet, 0, metrics.MetricTypeNameRaw,
			metrics.ConvertMapToTags(map[string]string{
				metricTagMetricName: targetMetric,
			})...)
		return &pluginapi.ThresholdMetResponse{
			MetType: pluginapi.ThresholdMetType_NOT_MET,
		}, nil
	}

	_ = p.emitter.StoreFloat64(metricsNameThresholdMet, 1, metrics.MetricTypeNameRaw,
		metrics.ConvertMapToTags(map[string]string{
			metricTagMetricName: targetMetric,
		})...)

	return &pluginapi.ThresholdMetResponse{
		ThresholdValue:    1,
		ObservedValue:     1,
		ThresholdOperator: pluginapi.ThresholdOperator_GREATER_THAN,
		MetType:           pluginapi.ThresholdMetType_HARD_MET,
		EvictionScope:     targetMetric,
		Condition: &pluginapi.Condition{
			ConditionType: pluginapi.ConditionType_NODE_CONDITION,
			Effects:       []string{string(v1.TaintEffectNoSchedule)},
			ConditionName: evictionConditionCPUUsagePressure,
			MetCondition:  true,
		},
	}, nil
}

func (p *NumaCPUPressureEviction) GetTopEvictionPods(_ context.Context, _ *pluginapi.GetTopEvictionPodsRequest,
) (*pluginapi.GetTopEvictionPodsResponse, error) {
	return &pluginapi.GetTopEvictionPodsResponse{}, nil
}

// 1 collect numa / pod level metrics
// 2 update node / numa overload status
func (p *NumaCPUPressureEviction) update(_ context.Context) {
	_ = p.emitter.StoreInt64(metricsNameCollectMetricsCalled, 1, metrics.MetricTypeNameRaw)

	p.Lock()
	defer p.Unlock()

	if !p.enabled {
		general.Infof("numa cpu pressure eviction is disabled")
		return
	}

	sharedPods, err := p.metaServer.GetPodList(context.Background(), func(pod *v1.Pod) bool {
		isValid, err := p.conf.QoSConfiguration.CheckSharedQoSForPod(pod)
		if err != nil {
			return false
		}
		return isValid
	})
	if err != nil {
		general.Warningf("failed to check shared qos on pods: %v", err)
		return
	}

	if len(sharedPods) == 0 {
		general.Warningf("find no shared_cores pods")
		return
	}

	general.Infof("start to collect numa pod metrics, shared pods count %v", len(sharedPods))
	numaSize := p.metaServer.CPUsPerNuma()
	// numa -> pod -> metric -> ring
	for numaID := 0; numaID < p.metaServer.NumNUMANodes; numaID++ {
		for _, pod := range sharedPods {
			podUID := string(pod.UID)
			for _, metricName := range targetMetrics {
				val, err := helper.GetPodMetric(p.metaServer.MetricsFetcher, p.emitter, pod, metricName, numaID)
				// calculate per core metric
				// todo refactor
				valPerCore := val / float64(numaSize)

				if err != nil {
					general.Warningf("failed to get pod metric, numa %v, pod %v, metric %v err: %v",
						numaID, pod.Name, metricName, err)
					continue
				}
				p.metricsHistory.Push(numaID, podUID, metricName, valPerCore)
				general.InfoS("Push pod metric", "metric", metricName, "numa", numaID, "pod", pod.Name, "value", valPerCore)
			}
		}
		// numa level sum
		for _, metricName := range targetMetrics {
			val := p.metaServer.AggregatePodNumaMetric(sharedPods, numaID, metricName, metric.AggregatorSum, filterPodContainer).Value
			// calculate per core metric
			// todo refactor
			valPerCore := val / float64(numaSize)

			p.metricsHistory.PushNuma(numaID, metricName, valPerCore)
			general.InfoS("Push numa metric", "metric", metricName, "numa", numaID, "value", valPerCore)
			_ = p.emitter.StoreFloat64(metricsNameNumaRaw, valPerCore, metrics.MetricTypeNameRaw,
				metrics.ConvertMapToTags(map[string]string{
					metricTagMetricName: metricName,
					metricTagNuma:       strconv.Itoa(numaID),
				})...)
		}
	}

	// update overload numa count and node overload
	p.overloadNumaCount = p.calOverloadNumaCount()

	//// collect numa usage ring
	//// numa -> usage
	//// only facing numa binding scenario
	//for numaID := 0; numaID < p.metaServer.NumNUMANodes; numaID++ {
	//	metricCPUUsageNuma, err := p.metaServer.GetNumaMetric(numaID, targetMetric)
	//	if err != nil {
	//		general.Errorf("GetNumaMetric for numa: %d failed with error: %v", numaID, err)
	//		continue
	//	}
	//	if p.numaMetricsHistory[numaID] == nil {
	//		p.numaMetricsHistory[numaID] = CreateMetricRing(p.evictionStrategy.MetricRingSize)
	//	}
	//	snapshot := &MetricSnapshot{
	//		Info: MetricInfo{
	//			Name:       targetMetric,
	//			Value:      metricCPUUsageNuma.Value,
	//			LowerBound: p.threshold,
	//			UpperBound: p.threshold,
	//		},
	//		Time: collectTime,
	//	}
	//	p.numaMetricsHistory[numaID].Push(snapshot)
	//}
}

func (p *NumaCPUPressureEviction) isNodeOverload() (nodeOverload bool) {
	numaCount := p.metaServer.NumNUMANodes
	overloadNumaCount := p.overloadNumaCount

	nodeOverload = overloadNumaCount == numaCount
	general.Infof("Update node overload %v", nodeOverload)

	return
}

func (p *NumaCPUPressureEviction) calOverloadNumaCount() (overloadNumaCount int) {
	thresholds := p.thresholds
	thresholdMetPercentage := p.loadConfig.ThresholdMetPercentage
	metricsHistory := p.metricsHistory
	emitter := p.emitter

	for numaID, numaHis := range metricsHistory.Inner {
		numaHisInner := numaHis[metricring.FakePodUID]
		var numaOver bool
		for metricName, metricRing := range numaHisInner {
			threshold, exist := thresholds[metricName]
			if !exist {
				general.Warningf("no threshold for metric %v", metricName)
				_ = emitter.StoreFloat64(metricNameMetricNoThreshold, 1, metrics.MetricTypeNameRaw,
					metrics.ConvertMapToTags(map[string]string{
						metricTagMetricName: metricName,
					})...)
				continue
			}
			// per numa metric
			overCount := metricRing.OverCount(threshold)
			// whether current numa is overload
			overRatio := float64(overCount) / float64(metricRing.MaxLen)

			numaOver = overRatio >= thresholdMetPercentage

			_ = emitter.StoreFloat64(metricNameNumaOverloadRatio, overRatio, metrics.MetricTypeNameRaw,
				metrics.ConvertMapToTags(map[string]string{
					metricTagMetricName: metricName,
					metricTagNuma:       strconv.Itoa(numaID),
					metricTagIsOverload: strconv.FormatBool(numaOver),
				})...)

			if numaOver {
				general.InfoS("numa is overload", "numaID", numaID, "metric", metricName, "overloadRatio", overRatio)
				break
			}
		}

		if numaOver {
			overloadNumaCount++
		}
	}
	_ = emitter.StoreInt64(metricNameNumaOverloadNumaCount, int64(overloadNumaCount), metrics.MetricTypeNameRaw)
	general.Infof("Update overload numa count %v", overloadNumaCount)
	return
}

func (p *NumaCPUPressureEviction) pickTopOverRatioNuma(metricName string, thresholds map[string]float64) (int, float64, error) {
	type NumaOverRatio struct {
		numaID        int
		overloadRatio float64
	}
	var numaOverRatios []NumaOverRatio

	for numaID, numaHis := range p.metricsHistory.Inner {
		numaHisInner := numaHis[metricring.FakePodUID]
		metricRing, ok := numaHisInner[metricName]
		if !ok {
			continue
		}
		threshold := thresholds[metricName]
		overCount := metricRing.OverCount(threshold)
		overRatio := float64(overCount) / float64(metricRing.MaxLen)
		numaOverRatios = append(numaOverRatios, NumaOverRatio{
			numaID:        numaID,
			overloadRatio: overRatio,
		})

	}

	sort.Slice(numaOverRatios, func(i, j int) bool {
		return numaOverRatios[i].overloadRatio > numaOverRatios[j].overloadRatio
	})

	for _, numa := range numaOverRatios {
		general.InfoS("Numa with overload ratio", "podName", numa.numaID, "overloadRatio", numa.overloadRatio)
	}

	if len(numaOverRatios) > 0 {
		return numaOverRatios[0].numaID, numaOverRatios[0].overloadRatio, nil
	}
	return -1, 0, fmt.Errorf("no valid numa found")
}

func (p *NumaCPUPressureEviction) pickTopOverRatioPod(numaID int, metricName string,
	thresholds map[string]float64, activePods []*v1.Pod) (*v1.Pod, float64, error) {
	type PodOverRatio struct {
		pod           *v1.Pod
		overloadRatio float64
	}
	var podOverRatios []PodOverRatio
	numaHis, ok := p.metricsHistory.Inner[numaID]
	if !ok {
		return nil, 0, fmt.Errorf("cannot find numa %v usage", numaID)
	}
	for _, pod := range activePods {
		podUID := string(pod.UID)
		podHis, existMetric := numaHis[podUID]
		if !existMetric {
			continue
		}
		metricRing, ok := podHis[metricName]
		if !ok {
			continue
		}
		threshold := thresholds[metricName]
		overCount := metricRing.OverCount(threshold)
		overRatio := float64(overCount) / float64(metricRing.MaxLen)
		podOverRatios = append(podOverRatios, PodOverRatio{
			pod:           pod,
			overloadRatio: overRatio,
		})
	}

	sort.Slice(podOverRatios, func(i, j int) bool {
		return podOverRatios[i].overloadRatio > podOverRatios[j].overloadRatio
	})

	for _, pod := range podOverRatios {
		general.InfoS("Pod with overload ratio", "podName", pod.pod.Name, "overloadRatio", pod.overloadRatio)
	}

	if len(podOverRatios) > 0 {
		_ = p.emitter.StoreFloat64(metricNameNumaOverloadTopPod, podOverRatios[0].overloadRatio, metrics.MetricTypeNameRaw,
			metrics.ConvertMapToTags(map[string]string{
				metricTagMetricName: metricName,
				metricTagNuma:       strconv.Itoa(numaID),
			})...)
		return podOverRatios[0].pod, podOverRatios[0].overloadRatio, nil
	}
	return nil, 0, fmt.Errorf("cannot find any pod to be evicted")
}

type LoadConfig struct {
	MetricRingSize          int
	ThresholdMetPercentage  float64
	NumaThresholdPercentage float64
	GracePeriod             int64
	ExpandFactor            float64
}
