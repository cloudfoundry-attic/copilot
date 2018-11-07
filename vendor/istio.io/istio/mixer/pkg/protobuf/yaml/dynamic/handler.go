// Copyright 2018 Istio Authors.
//
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

package dynamic

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	"github.com/pkg/errors"
	"go.uber.org/atomic"
	"google.golang.org/grpc"

	"istio.io/api/mixer/adapter/model/v1beta1"
	policypb "istio.io/api/policy/v1beta1"
	"istio.io/istio/mixer/pkg/adapter"
	protoyaml "istio.io/istio/mixer/pkg/protobuf/yaml"
	istiolog "istio.io/istio/pkg/log"
)

var (
	handlerLog = istiolog.RegisterScope("grpcAdapter", "dynamic grpc adapter debugging", 0)
)

type (
	// Handler is the dynamic handler implementation
	Handler struct {
		// Name is used for debug
		Name string

		// external grpc connection
		conn *grpc.ClientConn

		connConfig *policypb.Connection

		// svcMap is instance name to Svc mapping
		svcMap map[string]*Svc

		// n generates dedupeID when not given.
		n *atomic.Uint64
	}

	// Svc encapsulates abstract service
	Svc struct {
		Name       string
		Pkg        string
		MethodName string
		InputType  string
		OutputType string

		TemplateName string
		encoder      *messageEncoder
	}

	// TemplateConfig is template configuration
	TemplateConfig struct {
		Name         string
		TemplateName string
		FileDescSet  *descriptor.FileDescriptorSet
		Variety      v1beta1.TemplateVariety
	}

	// Codec in no-op on the way out and unmarshals using normal means
	// on the way in.
	Codec struct{}
)

// BuildHandler creates a dynamic handler object exposing specific handler interfaces.
func BuildHandler(name string, connConfig *policypb.Connection, sessionBased bool, adapterConfig proto.Marshaler,
	templateConfig []*TemplateConfig) (hh *Handler, err error) {

	// validate params
	if connConfig == nil || connConfig.Address == "" {
		return nil, errors.Errorf("empty connection address")
	}

	hh = &Handler{
		Name:       name,
		svcMap:     make(map[string]*Svc, len(templateConfig)),
		connConfig: connConfig,
		n:          &atomic.Uint64{},
	}

	// RemoteAdapterSvc is bound to a template
	// however the access is via instance name.
	tmplMap := make(map[string]*Svc, len(templateConfig))

	var svc *Svc
	for _, tc := range templateConfig {
		svc = tmplMap[tc.TemplateName]
		if svc == nil {
			if svc, err = RemoteAdapterSvc("Handle",
				protoyaml.NewResolver(tc.FileDescSet), sessionBased, adapterConfig, tc.TemplateName); err != nil {
				return nil, err
			}
			tmplMap[tc.TemplateName] = svc
		}
		hh.svcMap[tc.Name] = svc
	}

	hh.n.Store(rand.Uint64())
	if err = hh.connect(); err != nil {
		return nil, err
	}

	return hh, nil
}

// Close implements io.Closer api
func (h *Handler) Close() error {
	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

func (h *Handler) connect() (err error) {
	codec := grpc.CallCustomCodec(Codec{})
	// TODO add simple secure option
	if h.conn, err = grpc.Dial(h.connConfig.GetAddress(), grpc.WithInsecure(),
		grpc.WithDefaultCallOptions(codec)); err != nil {
		handlerLog.Errorf("Unable to connect to:%s %v", h.connConfig.GetAddress(), err)
		return errors.WithStack(err)
	}

	handlerLog.Infof("Connected to: %s", h.connConfig.GetAddress())
	return nil
}

func (h *Handler) handleRemote(ctx context.Context, qr proto.Marshaler,
	dedupID string, resultPtr interface{}, encodedInstances ...*adapter.EncodedInstance) error {
	if len(encodedInstances) == 0 {
		return errors.New("internal: no instances sent")
	}
	svc := h.svcMap[encodedInstances[0].Name]
	if svc == nil {
		return errors.Errorf("unable to find instance: %s", encodedInstances[0].Name)
	}

	for i := 1; i < len(encodedInstances); i++ {
		if tsvc := h.svcMap[encodedInstances[i].Name]; tsvc == nil || tsvc.TemplateName != svc.TemplateName {
			return errors.Errorf("template mismatch for %s. got: %s, want: %s", encodedInstances[i].Name,
				tsvc.TemplateName, svc.TemplateName)
		}
	}

	if dedupID == "" {
		dedupID = dedupeString(h.n.Load())
		h.n.Inc()
	}

	ba, err := svc.encodeRequest(qr, dedupID, encodedInstances...)
	if err != nil {
		return err
	}

	if err := h.conn.Invoke(ctx, svc.GrpcPath(), ba, resultPtr); err != nil {
		handlerLog.Warnf("unable to connect to:%s, %s", svc.GrpcPath(), h.connConfig.Address)
		return errors.WithStack(err)
	}

	return nil
}

var _ adapter.RemoteCheckHandler = &Handler{}

// HandleRemoteCheck implements adapter.RemoteCheckHandler api
func (h *Handler) HandleRemoteCheck(ctx context.Context, encodedInstance *adapter.EncodedInstance) (*adapter.CheckResult, error) {
	result := &v1beta1.CheckResult{}
	if err := h.handleRemote(ctx, nil, "", result, encodedInstance); err != nil {
		return nil, err
	}

	return &adapter.CheckResult{
		Status:        result.Status,
		ValidUseCount: result.ValidUseCount,
		ValidDuration: result.ValidDuration,
	}, nil
}

var _ adapter.RemoteReportHandler = &Handler{}

// HandleRemoteReport implements adapter.RemoteReportHandler api
func (h *Handler) HandleRemoteReport(ctx context.Context, encodedInstances []*adapter.EncodedInstance) error {
	// ReportResult is empty and it is ignored
	return h.handleRemote(ctx, nil, "", &v1beta1.ReportResult{}, encodedInstances...)
}

var _ adapter.RemoteQuotaHandler = &Handler{}

// HandleRemoteQuota implements adapter.RemoteQuotaHandler api
func (h *Handler) HandleRemoteQuota(ctx context.Context, encodedInstance *adapter.EncodedInstance,
	args *adapter.QuotaArgs) (*adapter.QuotaResult, error) {
	result := &v1beta1.QuotaResult{}
	qr := &v1beta1.QuotaRequest{
		Quotas: map[string]v1beta1.QuotaRequest_QuotaParams{
			encodedInstance.Name: {
				Amount:     args.QuotaAmount,
				BestEffort: args.BestEffort,
			},
		},
	}

	if err := h.handleRemote(ctx, qr, args.DeduplicationID, result, encodedInstance); err != nil {
		return nil, err
	}

	qRes, found := result.Quotas[encodedInstance.Name]
	if !found {
		return nil, errors.Errorf("remote server did not respond with the requested quota '%s'. quotas granted: %v", encodedInstance.Name, result.Quotas)
	}

	return &adapter.QuotaResult{
		ValidDuration: qRes.ValidDuration,
		Amount:        qRes.GrantedAmount,
	}, nil
}

// RemoteAdapterSvc returns RemoteAdapter service
func RemoteAdapterSvc(namePrefix string, res protoyaml.Resolver, sessionBased bool,
	adapterConfig proto.Marshaler, templateName string) (*Svc, error) {
	svc, pkg := res.ResolveService(namePrefix)

	if svc == nil {
		return nil, errors.Errorf("no service matched prefix:'%s'", namePrefix)
	}

	if len(svc.Method) == 0 {
		return nil, errors.Errorf("no methods defined in service:'%s'", svc.GetName())
	}
	method := svc.GetMethod()[0]
	instBuilder := NewEncoderBuilder(res, nil, true)
	me, err := buildRequestEncoder(instBuilder, method.GetInputType(), sessionBased, adapterConfig)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &Svc{
		Name:         svc.GetName(),
		Pkg:          pkg,
		MethodName:   method.GetName(),
		InputType:    method.GetInputType(),
		OutputType:   method.GetOutputType(),
		encoder:      me,
		TemplateName: templateName,
	}, nil
}

type staticBag struct {
	v map[string]interface{}
}

func (eb staticBag) Get(name string) (interface{}, bool) {
	v, found := eb.v[name]
	return v, found
}

func (eb staticBag) Names() []string {
	ret := make([]string, len(eb.v))
	for k := range eb.v {
		ret = append(ret, k)
	}
	return ret
}
func (eb staticBag) Done() {}
func (eb staticBag) Contains(key string) bool {
	_, found := eb.v[key]
	return found
}
func (eb staticBag) String() string { return fmt.Sprintf("%v", eb.v) }

const quotaRequestAttrName = "-quota-request-"
const dedupeAttrName = "-dedup_id-"
const instanceAttrName = "-pre-encoded-instance-"

// buildRequestEncoder is based on code gen check code gen that includes 3/4 fields.
func buildRequestEncoder(b *Builder, inputMsg string, sessionBased bool, adapterConfig proto.Marshaler) (*messageEncoder, error) {
	inputData := map[string]interface{}{
		"instance": &staticAttributeEncoder{ // check and quota have instance
			attrName: instanceAttrName,
		},
		"instances": &staticAttributeEncoder{ // report has instances
			attrName: instanceAttrName,
		},
		"dedup_id": &staticAttributeEncoder{
			attrName: dedupeAttrName,
		},
		"quota_request": &staticAttributeEncoder{
			attrName: quotaRequestAttrName,
		},
	}
	if !sessionBased {
		if adapterConfig != nil &&
			(reflect.ValueOf(adapterConfig).Kind() != reflect.Ptr || !reflect.ValueOf(adapterConfig).IsNil()) {
			encodedData, err := adapterConfig.Marshal()
			if err != nil {
				return nil, err // Any.Marshal() never returns an error.
			}
			inputData["adapter_config"] = &staticEncoder{
				encodedData:   encodedData,
				includeLength: true,
			}
		}
	}

	e, err := b.Build(inputMsg, inputData)
	if err != nil {
		return nil, err
	}
	me := e.(messageEncoder)
	return &me, nil
}

func dedupeString(d uint64) string {
	return fmt.Sprintf("%d", d)
}

// GrpcPath returns grpc POST url for this service.
func (s Svc) GrpcPath() string {
	return fmt.Sprintf("/%s.%s/%s", s.Pkg, s.Name, s.MethodName)
}

// encodeRequest encodes request using the message encoder. It supports Check, Report and Quota.
// If qr arg is required when using quota adapter.
func (s Svc) encodeRequest(qr proto.Marshaler, dedupID string, encodedInstances ...*adapter.EncodedInstance) ([]byte, error) {
	// at most 3 attributes at one time.
	bag := staticBag{v: make(map[string]interface{}, 3)}
	size := 0
	for _, ei := range encodedInstances {
		size += len(ei.Data)
	}
	// every fields needs up to 3 bytes
	size += len(s.encoder.fields) * 3
	ba := make([]byte, 0, size)

	instField := s.encoder.fields[0]
	for _, ei := range encodedInstances {
		var err error
		bag.v[instanceAttrName] = ei.Data
		ba, err = instField.Encode(&bag, ba)
		if err != nil {
			return nil, errors.Errorf("fieldEncoder: %s - %v", instField.name, err)
		}
	}

	bag.v[dedupeAttrName] = []byte(dedupID)
	if qr != nil {
		var encodedData []byte
		var err error
		if encodedData, err = qr.Marshal(); err != nil {
			return nil, errors.WithStack(err)
		}
		bag.v[quotaRequestAttrName] = encodedData
	}

	for _, f := range s.encoder.fields[1:] {
		var err error
		ba, err = f.Encode(&bag, ba)
		if err != nil {
			return nil, errors.Errorf("fieldEncoder: %s - %v", f.name, err)
		}
	}
	return ba, nil
}

// Marshal does a noo-op marshal if input in bytes, otherwise it is an error.
func (Codec) Marshal(v interface{}) ([]byte, error) {
	if ba, ok := v.([]byte); ok {
		return ba, nil
	}

	return nil, fmt.Errorf("unable to marshal type:%T, want []byte", v)
}

// Unmarshal delegates to standard proto Unmarshal
func (Codec) Unmarshal(data []byte, v interface{}) error {
	if um, ok := v.(proto.Unmarshaler); ok {
		return um.Unmarshal(data)
	}

	return fmt.Errorf("unable to unmarshal type:%T, %v", v, v)
}

// String returns name of the codec.
func (Codec) String() string {
	return "bytes-out-proto-in-codec"
}
