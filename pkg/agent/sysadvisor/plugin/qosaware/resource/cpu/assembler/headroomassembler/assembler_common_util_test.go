package headroomassembler

//
//import (
//	info "github.com/google/cadvisor/info/v1"
//	"github.com/kubewharf/katalyst-core/pkg/agent/sysadvisor/metacache"
//	"github.com/kubewharf/katalyst-core/pkg/agent/sysadvisor/plugin/qosaware/resource/cpu/region"
//	"github.com/kubewharf/katalyst-core/pkg/config"
//	"github.com/kubewharf/katalyst-core/pkg/metaserver"
//	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent"
//	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric"
//	metrictypes "github.com/kubewharf/katalyst-core/pkg/metaserver/agent/metric/types"
//	"github.com/kubewharf/katalyst-core/pkg/metaserver/agent/pod"
//	"github.com/kubewharf/katalyst-core/pkg/metrics"
//	metricspool "github.com/kubewharf/katalyst-core/pkg/metrics/metrics-pool"
//	"github.com/kubewharf/katalyst-core/pkg/util/machine"
//	"github.com/stretchr/testify/require"
//	"io/ioutil"
//	v1 "k8s.io/api/core/v1"
//	"os"
//	"reflect"
//	"testing"
//)
//
//func TestHeadroomAssemblerCommon_getReclaimNUMARequest(t *testing.T) {
//	type fields struct {
//		conf               *config.Configuration
//		regionMap          *map[string]region.QoSRegion
//		reservedForReclaim *map[int]int
//		numaAvailable      *map[int]int
//		nonBindingNumas    *machine.CPUSet
//		metaReader         metacache.MetaReader
//		metaServer         *metaserver.MetaServer
//		emitter            metrics.MetricEmitter
//	}
//	type args struct {
//		bindingNUMAs []int
//	}
//	tests := []struct {
//		name              string
//		fields            fields
//		args              args
//		wantBindingRes    map[int]float64
//		wantNonBindingRes float64
//		wantErr           bool
//	}{
//		// TODO: Add test cases.
//		{
//			name: "",
//			fields: fields{
//				regionMap:  &map[string]region.QoSRegion{},
//				//metaReader: metacache.MetaCacheImp{},
//			},
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//
//			err, conf := genConf(t)
//
//			metricsFetcher := metric.NewFakeMetricsFetcher(metrics.DummyMetrics{})
//			metaCache, err := metacache.NewMetaCacheImp(conf, metricspool.DummyMetricsEmitterPool{}, metricsFetcher)
//			require.NoError(t, err)
//
//			metaServer := generateTestMetaServer(t, nil, nil, metricsFetcher)
//
//			ha := &HeadroomAssemblerCommon{
//				conf:               tt.fields.conf,
//				regionMap:          tt.fields.regionMap,
//				reservedForReclaim: tt.fields.reservedForReclaim,
//				numaAvailable:      tt.fields.numaAvailable,
//				nonBindingNumas:    tt.fields.nonBindingNumas,
//				metaReader:         metaCache,
//				metaServer:         tt.fields.metaServer,
//				emitter:            metrics.DummyMetrics{},
//			}
//			gotBindingRes, gotNonBindingRes, err := ha.getReclaimNUMARequest(tt.args.bindingNUMAs)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("getReclaimNUMARequest() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(gotBindingRes, tt.wantBindingRes) {
//				t.Errorf("getReclaimNUMARequest() gotBindingRes = %v, want %v", gotBindingRes, tt.wantBindingRes)
//			}
//			if gotNonBindingRes != tt.wantNonBindingRes {
//				t.Errorf("getReclaimNUMARequest() gotNonBindingRes = %v, want %v", gotNonBindingRes, tt.wantNonBindingRes)
//			}
//		})
//	}
//}
//
//func genConf(t *testing.T) (error, *config.Configuration) {
//	ckDir, err := ioutil.TempDir("", "checkpoint-TestHeadroomAssemblerCommon_GetHeadroom")
//	require.NoError(t, err)
//	defer os.RemoveAll(ckDir)
//
//	sfDir, err := ioutil.TempDir("", "statefile")
//	require.NoError(t, err)
//	defer os.RemoveAll(sfDir)
//	conf := generateTestConfiguration(t, ckDir, sfDir)
//	return err, conf
//}
