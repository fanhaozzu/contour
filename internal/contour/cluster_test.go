// Copyright © 2018 Heptio
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package contour

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/auth"
	"github.com/envoyproxy/go-control-plane/envoy/api/v2/core"
	"github.com/gogo/protobuf/proto"
)

// TODO(dfc) clean up these tests with helpers for the want: fixtures.

func TestClusterCacheRecomputeService(t *testing.T) {
	tests := map[string]struct {
		oldObj *v1.Service
		newObj *v1.Service
		want   []proto.Message
	}{
		"add unnamed service": {
			oldObj: nil,
			newObj: service("default", "kuard",
				v1.ServicePort{
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/443",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				},
			},
		},
		"name previously unnamed port": {
			oldObj: service("default", "kuard",
				v1.ServicePort{
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			newObj: service("default", "kuard",
				v1.ServicePort{
					Name:       "https",
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/443",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/https",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				}, &v2.Cluster{
					Name: "default/kuard/https",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/https",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				},
			},
		},
		"remove name from port": {
			oldObj: service("default", "kuard",
				v1.ServicePort{
					Name:       "https",
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			newObj: service("default", "kuard",
				v1.ServicePort{
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/443",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				},
			},
		},
		"update service port": {
			oldObj: service("default", "kuard",
				v1.ServicePort{
					Name:       "http",
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			),
			newObj: service("default", "kuard",
				v1.ServicePort{
					Name:       "http",
					Protocol:   "TCP",
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/8080",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/http",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				}, &v2.Cluster{
					Name: "default/kuard/http",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/http",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				},
			},
		},
		"remove named service port": {
			oldObj: service("default", "kuard",
				v1.ServicePort{
					Name:       "http",
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
				v1.ServicePort{
					Name:       "https",
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			newObj: service("default", "kuard",
				v1.ServicePort{
					Name:       "https",
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/443",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/https",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				}, &v2.Cluster{
					Name: "default/kuard/https",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/https",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				},
			},
		},
		"update, remove, and rename a named port": {
			oldObj: service("default", "kuard",
				v1.ServicePort{
					Name:       "http",
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
				v1.ServicePort{
					Name:       "https",
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			),
			newObj: service("default", "kuard",
				v1.ServicePort{
					Protocol:   "TCP",
					Port:       443,
					TargetPort: intstr.FromInt(8000),
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/443",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				},
			},
		},
		"issue#243": {
			oldObj: nil,
			newObj: service("default", "kuard",
				v1.ServicePort{
					Protocol:   "TCP",
					Port:       80,
					TargetPort: intstr.FromInt(8080),
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/80",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard",
					},
					ConnectTimeout: 250 * time.Millisecond,
					LbPolicy:       v2.Cluster_ROUND_ROBIN,
				},
			},
		},
		"h2c upstream": {
			oldObj: nil,
			newObj: serviceWithAnnotations(
				"default",
				"kuard",
				map[string]string{
					fmt.Sprintf("%s.%s", annotationUpstreamProtocol, "h2c"): "80,http",
				},
				v1.ServicePort{
					Protocol: "TCP",
					Name:     "http",
					Port:     80,
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/80",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/http",
					},
					ConnectTimeout:       250 * time.Millisecond,
					LbPolicy:             v2.Cluster_ROUND_ROBIN,
					Http2ProtocolOptions: &core.Http2ProtocolOptions{},
				},
				&v2.Cluster{
					Name: "default/kuard/http",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/http",
					},
					ConnectTimeout:       250 * time.Millisecond,
					LbPolicy:             v2.Cluster_ROUND_ROBIN,
					Http2ProtocolOptions: &core.Http2ProtocolOptions{},
				},
			},
		},
		"h2 upstream": {
			oldObj: nil,
			newObj: serviceWithAnnotations(
				"default",
				"kuard",
				map[string]string{
					fmt.Sprintf("%s.%s", annotationUpstreamProtocol, "h2"): "443,https",
				},
				v1.ServicePort{
					Protocol: "TCP",
					Name:     "https",
					Port:     443,
				},
			),
			want: []proto.Message{
				&v2.Cluster{
					Name: "default/kuard/443",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/https",
					},
					ConnectTimeout:       250 * time.Millisecond,
					LbPolicy:             v2.Cluster_ROUND_ROBIN,
					Http2ProtocolOptions: &core.Http2ProtocolOptions{},
					TlsContext: &auth.UpstreamTlsContext{
						CommonTlsContext: &auth.CommonTlsContext{
							AlpnProtocols: []string{"h2"},
						},
					},
				},
				&v2.Cluster{
					Name: "default/kuard/https",
					Type: v2.Cluster_EDS,
					EdsClusterConfig: &v2.Cluster_EdsClusterConfig{
						EdsConfig:   apiconfigsource("contour"), // hard coded by initconfig
						ServiceName: "default/kuard/https",
					},
					ConnectTimeout:       250 * time.Millisecond,
					LbPolicy:             v2.Cluster_ROUND_ROBIN,
					Http2ProtocolOptions: &core.Http2ProtocolOptions{},
					TlsContext: &auth.UpstreamTlsContext{
						CommonTlsContext: &auth.CommonTlsContext{
							AlpnProtocols: []string{"h2"},
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var cc ClusterCache
			cc.recomputeService(tc.oldObj, tc.newObj)
			got := contents(&cc)
			sort.Stable(clusterByName(got))
			if !reflect.DeepEqual(tc.want, got) {
				t.Fatalf("expected:\n%v\ngot:\n%v\n", tc.want, got)
			}
		})
	}
}

func TestServiceName(t *testing.T) {
	tests := map[string]struct {
		meta metav1.ObjectMeta
		name string
		want string
	}{
		"named service": {
			meta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "kuard",
			},
			name: "http",
			want: "default/kuard/http",
		},
		"unnamed service": {
			meta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "kuard",
			},
			want: "default/kuard",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := servicename(tc.meta, tc.name)
			if got != tc.want {
				t.Fatalf("servicename(%#v, %q): want %q, got %q", tc.meta, tc.name, tc.want, got)
			}
		})
	}
}

type clusterByName []proto.Message

func (c clusterByName) Len() int           { return len(c) }
func (c clusterByName) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c clusterByName) Less(i, j int) bool { return c[i].(*v2.Cluster).Name < c[j].(*v2.Cluster).Name }
