package strategy

import (
	"github.com/kubewharf/katalyst-core/pkg/config/agent/dynamic/metricthreshold"
	"github.com/kubewharf/katalyst-core/pkg/consts"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric"
	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric/types"
	"github.com/kubewharf/katalyst-core/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_expandThresholds(t *testing.T) {
	type args struct {
		thresholds   map[string]float64
		expandFactor float64
	}
	tests := []struct {
		name string
		args args
		want map[string]float64
	}{
		{
			name: "test",
			args: args{
				thresholds: map[string]float64{
					"1": 1,
					"2": 2,
					"3": 3,
				},
				expandFactor: 2,
			},
			want: map[string]float64{
				"1": 2,
				"2": 4,
				"3": 6,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, expandThresholds(tt.args.thresholds, tt.args.expandFactor), "expandThresholds(%v, %v)", tt.args.thresholds, tt.args.expandFactor)
		})
	}
}

func Test_getCpuCodeAndIsVm(t *testing.T) {
	type args struct {
		metricsFetcher types.MetricsFetcher
	}
	tests := []struct {
		name            string
		args            args
		setFakeMetric   func(store *metric.FakeMetricsFetcher)
		wantCpuCodeName string
		wantIsVm        bool
	}{
		{
			name: "test",
			args: args{
				metricsFetcher: metric.NewFakeMetricsFetcher(metrics.DummyMetrics{}),
			},
			setFakeMetric: func(store *metric.FakeMetricsFetcher) {
				store.SetByStringIndex(consts.MetricCPUCodeName, "abc")
				store.SetByStringIndex(consts.MetricInfoIsVM, "true")
			},
			wantCpuCodeName: "abc",
			wantIsVm:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metricsFetcher := metric.NewFakeMetricsFetcher(metrics.DummyMetrics{})
			store := metricsFetcher.(*metric.FakeMetricsFetcher)
			tt.setFakeMetric(store)
			gotCpuCodeName, gotIsVm := getCpuCodeAndIsVm(store)
			assert.Equalf(t, tt.wantCpuCodeName, gotCpuCodeName, "getCpuCodeAndIsVm(%v)", tt.args.metricsFetcher)
			assert.Equalf(t, tt.wantIsVm, gotIsVm, "getCpuCodeAndIsVm(%v)", tt.args.metricsFetcher)
		})
	}
}

func Test_convertThreshold(t *testing.T) {
	type args struct {
		origin map[string]float64
	}
	tests := []struct {
		name string
		args args
		want map[string]float64
	}{
		{
			name: "test",
			args: args{
				origin: map[string]float64{
					"cpu_usage_threshold": 1,
					"cpu_load_threshold":  2,
					"xxx":                 3,
				},
			},
			want: map[string]float64{
				consts.MetricCPUUsageContainer: 1,
				consts.MetricLoad1MinContainer: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, convertThreshold(tt.args.origin), "convertThreshold(%v)", tt.args.origin)
		})
	}
}

func Test_getOverLoadThreshold(t *testing.T) {
	type args struct {
		globalThresholds *metricthreshold.MetricThreshold
		cpuCode          string
		isVM             bool
	}
	tests := []struct {
		name string
		args args
		want map[string]float64
	}{
		{
			name: "test",
			args: args{
				globalThresholds: &metricthreshold.MetricThreshold{
					Threshold: map[string]map[bool]map[string]float64{
						"abc": {
							true: {
								"cpu_usage_threshold": 1,
								"cpu_load_threshold":  2,
							},
							false: {
								"cpu_usage_threshold": 3,
								"cpu_load_threshold":  4,
							},
						},
						"def": {
							true: {
								"cpu_usage_threshold": 5,
								"cpu_load_threshold":  6,
							},
							false: {
								"cpu_usage_threshold": 7,
								"cpu_load_threshold":  8,
							},
						},
					},
				},
				cpuCode: "abc",
				isVM:    true,
			},
			want: map[string]float64{
				"cpu_usage_threshold": 1,
				"cpu_load_threshold":  2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, getOverLoadThreshold(tt.args.globalThresholds, tt.args.cpuCode, tt.args.isVM), "getOverLoadThreshold(%v, %v, %v)", tt.args.globalThresholds, tt.args.cpuCode, tt.args.isVM)
		})
	}
}
