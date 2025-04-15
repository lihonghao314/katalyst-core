package trombe

import (
	"context"
	"testing"

	"github.com/kubewharf/katalyst-core/pkg/agent/qrm-plugins/commonstate"
	"github.com/kubewharf/katalyst-core/pkg/agent/qrm-plugins/cpu/dynamicpolicy/cpueviction/trombe/metricring"
	"github.com/kubewharf/katalyst-core/pkg/config"
	"github.com/kubewharf/katalyst-core/pkg/consts"
	"github.com/kubewharf/katalyst-core/pkg/metaserver"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric"
	metrictypes "github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric/types"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/pod"
	"github.com/kubewharf/katalyst-core/pkg/metrics"
	"github.com/kubewharf/katalyst-core/pkg/util/machine"
	utilmetric "github.com/kubewharf/katalyst-core/pkg/util/metric"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultMetricRingSize                           = 1
	defaultCPUPressureEvictionPodGracePeriodSeconds = -1
	defaultLoadUpperBoundRatio                      = 1.8
	defaultLoadLowerBoundRatio                      = 1.0
	defaultLoadThresholdMetPercentage               = 0.8
	defaultReservedForAllocate                      = "4"
	defaultReservedForReclaim                       = "4"
	defaultReservedForSystem                        = 0
)

var (
	cpuTopology, _ = machine.GenerateDummyCPUTopology(16, 2, 4)
	conf           = makeConf(defaultMetricRingSize, int64(defaultCPUPressureEvictionPodGracePeriodSeconds),
		defaultLoadUpperBoundRatio, defaultLoadLowerBoundRatio,
		defaultLoadThresholdMetPercentage, defaultReservedForReclaim,
		defaultReservedForAllocate, defaultReservedForSystem)
)

func makeMetaServer(metricsFetcher metrictypes.MetricsFetcher, cpuTopology *machine.CPUTopology, podList []*v1.Pod) *metaserver.MetaServer {
	metaServer := &metaserver.MetaServer{
		MetaAgent: &agent.MetaAgent{},
	}

	metaServer.MetricsFetcher = metricsFetcher
	metaServer.KatalystMachineInfo = &machine.KatalystMachineInfo{
		CPUTopology: cpuTopology,
	}

	metaServer.PodFetcher = &pod.PodFetcherStub{
		PodList: podList,
	}

	return metaServer
}

func makeConf(metricRingSize int, gracePeriod int64, loadUpperBoundRatio, loadLowerBoundRatio,
	loadThresholdMetPercentage float64, reservedForReclaim, reservedForAllocate string, reservedForSystem int,
) *config.Configuration {
	conf := config.NewConfiguration()
	conf.GetDynamicConfiguration().EnableLoadEviction = true
	conf.GetDynamicConfiguration().LoadMetricRingSize = metricRingSize
	conf.GetDynamicConfiguration().LoadUpperBoundRatio = loadUpperBoundRatio
	conf.GetDynamicConfiguration().LoadLowerBoundRatio = loadLowerBoundRatio
	conf.GetDynamicConfiguration().LoadThresholdMetPercentage = loadThresholdMetPercentage
	conf.GetDynamicConfiguration().CPUPressureEvictionConfiguration.GracePeriod = gracePeriod
	conf.GetDynamicConfiguration().ReservedResourceForAllocate = v1.ResourceList{
		v1.ResourceCPU: resource.MustParse(reservedForAllocate),
	}
	conf.GetDynamicConfiguration().MinReclaimedResourceForAllocate = v1.ResourceList{
		v1.ResourceCPU: resource.MustParse(reservedForReclaim),
	}
	conf.ReservedCPUCores = reservedForSystem
	conf.LoadPressureEvictionSkipPools = []string{
		commonstate.PoolNameReclaim,
		commonstate.PoolNameDedicated,
		commonstate.PoolNameFallback,
		commonstate.PoolNameReserve,
	}
	return conf
}

func TestNumaCPUPressureEviction_update(t *testing.T) {
	type fields struct {
		conf           *config.Configuration
		loadConfig     *LoadConfig
		metricsHistory *metricring.MetricHistory
		setFakeMetric  func(store *metric.FakeMetricsFetcher)
		podList        []*v1.Pod
	}

	type want struct {
		metricsHistory    *metricring.MetricHistory
		overloadNumaCount int
	}

	tests := []struct {
		name   string
		fields fields
		want   want
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			fields: fields{
				podList: []*v1.Pod{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "pod1",
							Namespace: "default",
							UID:       "pod1",
							Annotations: map[string]string{
								"katalyst.kubewharf.io/qos_level": "shared_cores",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name: "container",
								},
							},
						},
					},
				},
				conf: conf,
				loadConfig: &LoadConfig{
					MetricRingSize:         1,
					ThresholdMetPercentage: conf.DynamicAgentConfiguration.GetDynamicConfiguration().ThresholdMetPercentage,
					GracePeriod:            conf.DynamicAgentConfiguration.GetDynamicConfiguration().DeletionGracePeriod,
					ExpandFactor:           1.2,
				},
				setFakeMetric: func(store *metric.FakeMetricsFetcher) {
					store.SetContainerNumaMetric("pod1", "container", 0,
						consts.MetricsCPUUsageNUMAContainer, utilmetric.MetricData{Value: 2})
				},
				metricsHistory: metricring.NewMetricHistory(conf.DynamicAgentConfiguration.GetDynamicConfiguration().LoadMetricRingSize),
			},
			want: want{
				metricsHistory: &metricring.MetricHistory{
					Inner: map[int]map[string]map[string]*metricring.MetricRing{
						0: {
							metricring.FakePodUID: {
								consts.MetricsCPUUsageNUMAContainer: {
									MaxLen: 1,
									Queue: []*metricring.MetricSnapshot{
										{Info: metricring.MetricInfo{Name: consts.MetricsCPUUsageNUMAContainer, Value: 0.5}},
									},
									CurrentIndex: 0,
								},
							},
							"pod1": {
								consts.MetricsCPUUsageNUMAContainer: {
									MaxLen: 1,
									Queue: []*metricring.MetricSnapshot{
										{Info: metricring.MetricInfo{Name: consts.MetricsCPUUsageNUMAContainer, Value: 0.5}},
									},
									CurrentIndex: 0,
								},
							},
						},
						1: {
							metricring.FakePodUID: {
								consts.MetricsCPUUsageNUMAContainer: {
									MaxLen:       1,
									Queue:        []*metricring.MetricSnapshot{nil},
									CurrentIndex: 0,
								},
							},
						},
						2: {
							metricring.FakePodUID: {
								consts.MetricsCPUUsageNUMAContainer: {
									MaxLen:       1,
									Queue:        []*metricring.MetricSnapshot{nil},
									CurrentIndex: 0,
								},
							},
						},
						3: {
							metricring.FakePodUID: {
								consts.MetricsCPUUsageNUMAContainer: {
									MaxLen:       1,
									Queue:        []*metricring.MetricSnapshot{nil},
									CurrentIndex: 0,
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricsFetcher := metric.NewFakeMetricsFetcher(metrics.DummyMetrics{})

			metaServer := makeMetaServer(metricsFetcher, cpuTopology, tt.fields.podList)

			store := metricsFetcher.(*metric.FakeMetricsFetcher)
			tt.fields.setFakeMetric(store)

			p := &NumaCPUPressureEviction{
				metaServer:     metaServer,
				conf:           tt.fields.conf,
				loadConfig:     tt.fields.loadConfig,
				metricsHistory: tt.fields.metricsHistory,
				emitter:        metrics.DummyMetrics{},
			}

			p.update(context.TODO())
			if CompareMetricHistory(p.metricsHistory, tt.want.metricsHistory) {
				t.Errorf("update() got = %v, want %v", p.metricsHistory, tt.want)
			}
		})
	}
}

func CompareMetricHistory(mh1, mh2 *metricring.MetricHistory) bool {
	if mh1.RingSize != mh2.RingSize {
		return false
	}

	if len(mh1.Inner) != len(mh2.Inner) {
		return false
	}

	for numaID, podMap1 := range mh1.Inner {
		podMap2, exists := mh2.Inner[numaID]
		if !exists {
			return false
		}

		if len(podMap1) != len(podMap2) {
			return false
		}

		for podUID, metricMap1 := range podMap1 {
			metricMap2, exists := podMap2[podUID]
			if !exists {
				return false
			}

			if len(metricMap1) != len(metricMap2) {
				return false
			}

			for metricName, ring1 := range metricMap1 {
				ring2, exists := metricMap2[metricName]
				if !exists {
					return false
				}

				if ring1.MaxLen != ring2.MaxLen || ring1.CurrentIndex != ring2.CurrentIndex {
					return false
				}

				if len(ring1.Queue) != len(ring2.Queue) {
					return false
				}

				for i, snapshot1 := range ring1.Queue {
					snapshot2 := ring2.Queue[i]
					if snapshot1.Info.Name != snapshot2.Info.Name || snapshot1.Info.Value != snapshot2.Info.Value {
						return false
					}
				}
			}
		}
	}

	return true
}
