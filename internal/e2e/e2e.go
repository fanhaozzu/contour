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

package e2e

// grpc helpers

import (
	"net"
	"sync"
	"testing"

	"github.com/envoyproxy/go-control-plane/envoy/api/v2"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/heptio/contour/internal/contour"
	cgrpc "github.com/heptio/contour/internal/grpc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	googleApis   = "type.googleapis.com/"
	typePrefix   = googleApis + "envoy.api.v2."
	endpointType = typePrefix + "ClusterLoadAssignment"
	clusterType  = typePrefix + "Cluster"
	routeType    = typePrefix + "RouteConfiguration"
	listenerType = typePrefix + "Listener"
)

type testWriter struct {
	*testing.T
}

func (t *testWriter) Write(buf []byte) (int, error) {
	t.Logf("%s", buf)
	return len(buf), nil
}

func setup(t *testing.T) (cache.ResourceEventHandler, *grpc.ClientConn, func()) {
	log := logrus.New()
	log.Out = &testWriter{t}

	tr := &contour.Translator{
		FieldLogger: log,
	}
	et := &contour.EndpointsTranslator{
		FieldLogger: log,
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	check(t, err)
	var wg sync.WaitGroup
	wg.Add(1)
	srv := cgrpc.NewAPI(log, tr, et)
	go func() {
		defer wg.Done()
		srv.Serve(l)
	}()
	cc, err := grpc.Dial(l.Addr().String(), grpc.WithInsecure())
	check(t, err)

	reh := &resourceEventHandler{
		Translator:          tr,
		EndpointsTranslator: et,
	}

	return reh, cc, func() {
		// close client connection
		cc.Close()

		// shut down listener, stop server and wait for it to stop
		l.Close()
		srv.Stop()
		wg.Wait()
	}
}

// resourceEventHandler composes a contour.Translator and a contour.EndpointsTranslator
// into a single ResourceEventHandler type.
type resourceEventHandler struct {
	*contour.Translator
	*contour.EndpointsTranslator
}

func (r *resourceEventHandler) OnAdd(obj interface{}) {
	switch obj.(type) {
	case *v1.Endpoints:
		r.EndpointsTranslator.OnAdd(obj)
	default:
		r.Translator.OnAdd(obj)
	}
}

func (r *resourceEventHandler) OnUpdate(oldObj, newObj interface{}) {
	switch newObj.(type) {
	case *v1.Endpoints:
		r.EndpointsTranslator.OnUpdate(oldObj, newObj)
	default:
		r.Translator.OnUpdate(oldObj, newObj)
	}
}

func (r *resourceEventHandler) OnDelete(obj interface{}) {
	switch obj.(type) {
	case *v1.Endpoints:
		r.EndpointsTranslator.OnDelete(obj)
	default:
		r.Translator.OnDelete(obj)
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func any(t *testing.T, pb proto.Message) types.Any {
	t.Helper()
	any, err := types.MarshalAny(pb)
	if err != nil {
		t.Fatal(err)
	}
	return *any
}

func assertEqual(t *testing.T, want, got *v2.DiscoveryResponse) {
	t.Helper()
	m := proto.TextMarshaler{Compact: true, ExpandAny: true}
	a := m.Text(want)
	b := m.Text(got)
	if a != b {
		m := proto.TextMarshaler{
			Compact:   false,
			ExpandAny: true,
		}
		t.Fatalf("\nexpected:\n%v\ngot:\n%v", m.Text(want), m.Text(got))
	}
}
