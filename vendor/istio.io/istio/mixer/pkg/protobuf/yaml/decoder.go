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

package yaml

import (
	"errors"
	"fmt"
	"math"

	"github.com/gogo/protobuf/protoc-gen-gogo/descriptor"
	multierror "github.com/hashicorp/go-multierror"

	"istio.io/istio/mixer/pkg/protobuf/yaml/wire"
)

type (
	// Decoder transforms protobuf-encoded bytes to attribute values.
	Decoder struct {
		resolver Resolver
		fields   map[wire.Number]*descriptor.FieldDescriptorProto
	}
)

// NewDecoder creates a decoder specific to a dynamic proto descriptor.
// Additionally, it takes as input an optional field mask to avoid decoding
// unused field values. Field mask is keyed by the message proto field names.
// A nil field mask implies all fields are decoded.
// This decoder is specialized to a single-level proto schema (no nested field dereferences
// in the resulting output).
func NewDecoder(resolver Resolver, msgName string, fieldMask map[string]bool) *Decoder {
	message := resolver.ResolveMessage(msgName)
	fields := make(map[wire.Number]*descriptor.FieldDescriptorProto)

	for _, f := range message.Field {
		if fieldMask == nil || fieldMask[f.GetName()] {
			fields[f.GetNumber()] = f
		}
	}

	return &Decoder{
		resolver: resolver,
		fields:   fields,
	}
}

// Decode function parses wire-encoded bytes to attribute values. The keys are field names
// in the message specified by the field mask.
func (d *Decoder) Decode(b []byte) (map[string]interface{}, error) {
	visitor := &decodeVisitor{
		decoder: d,
		out:     make(map[string]interface{}),
	}

	for len(b) > 0 {
		_, _, n := wire.ConsumeField(visitor, b)
		if n < 0 {
			return visitor.out, wire.ParseError(n)
		}
		b = b[n:]
	}

	return visitor.out, visitor.err
}

type decodeVisitor struct {
	decoder *Decoder
	out     map[string]interface{}
	err     error
}

func (dv *decodeVisitor) setValue(f *descriptor.FieldDescriptorProto, val interface{}) {
	if f.IsRepeated() {
		var arr []interface{}
		old, ok := dv.out[f.GetName()]
		if !ok {
			arr = make([]interface{}, 0, 1)
		} else {
			arr = old.([]interface{})
		}
		dv.out[f.GetName()] = append(arr, val)
	} else {
		dv.out[f.GetName()] = val
	}
}

// varint coalesces all primitive integers and enums to int64 type
func (dv *decodeVisitor) Varint(n wire.Number, v uint64) {
	f, exists := dv.decoder.fields[n]
	if !exists {
		return
	}
	var val interface{}
	switch f.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_BOOL:
		val = wire.DecodeBool(v)
	case descriptor.FieldDescriptorProto_TYPE_INT32,
		descriptor.FieldDescriptorProto_TYPE_INT64,
		descriptor.FieldDescriptorProto_TYPE_UINT32,
		descriptor.FieldDescriptorProto_TYPE_UINT64:
		val = int64(v)
	case descriptor.FieldDescriptorProto_TYPE_SINT32,
		descriptor.FieldDescriptorProto_TYPE_SINT64:
		val = int64(wire.DecodeZigZag(v))
	case descriptor.FieldDescriptorProto_TYPE_ENUM:
		val = int64(v)
	default:
		dv.err = multierror.Append(dv.err, fmt.Errorf("unexpected field type %q for varint encoding", f.GetType()))
		return
	}
	dv.setValue(f, val)
}

func (dv *decodeVisitor) Fixed32(n wire.Number, v uint32) {
	f, exists := dv.decoder.fields[n]
	if !exists {
		return
	}
	var val interface{}
	switch f.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_FIXED32:
		val = int64(v)
	case descriptor.FieldDescriptorProto_TYPE_SFIXED32:
		val = int64(v)
	case descriptor.FieldDescriptorProto_TYPE_FLOAT:
		val = math.Float32frombits(v)
	default:
		dv.err = multierror.Append(dv.err, fmt.Errorf("unexpected field type %q for fixed32 encoding", f.GetType()))
		return
	}
	dv.setValue(f, val)
}

func (dv *decodeVisitor) Fixed64(n wire.Number, v uint64) {
	f, exists := dv.decoder.fields[n]
	if !exists {
		return
	}
	var val interface{}
	switch f.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_FIXED64:
		val = int64(v)
	case descriptor.FieldDescriptorProto_TYPE_SFIXED64:
		val = int64(v)
	case descriptor.FieldDescriptorProto_TYPE_DOUBLE:
		val = math.Float64frombits(v)
	default:
		dv.err = multierror.Append(dv.err, fmt.Errorf("unexpected field type %q for fixed64 encoding", f.GetType()))
		return
	}
	dv.setValue(f, val)
}

type mapVisitor struct {
	desc       *descriptor.DescriptorProto
	key, value string
}

// TODO: only string maps are supported since mixer's IL only supports string maps
func (mv *mapVisitor) Varint(wire.Number, uint64)  {}
func (mv *mapVisitor) Fixed32(wire.Number, uint32) {}
func (mv *mapVisitor) Fixed64(wire.Number, uint64) {}
func (mv *mapVisitor) Bytes(n wire.Number, v []byte) {
	switch n {
	case mv.desc.Field[0].GetNumber():
		mv.key = string(v)
	case mv.desc.Field[1].GetNumber():
		mv.value = string(v)
	}
}

func (dv *decodeVisitor) Bytes(n wire.Number, v []byte) {
	f, exists := dv.decoder.fields[n]
	if !exists {
		return
	}
	switch f.GetType() {
	case descriptor.FieldDescriptorProto_TYPE_STRING:
		dv.setValue(f, string(v))
		return
	case descriptor.FieldDescriptorProto_TYPE_BYTES:
		dv.setValue(f, v)
		return
	case descriptor.FieldDescriptorProto_TYPE_MESSAGE:
		if isMap(dv.decoder.resolver, f) {
			// validate proto type to be map<string, string>
			mapType := dv.decoder.resolver.ResolveMessage(f.GetTypeName())

			if mapType == nil || len(mapType.Field) < 2 {
				dv.err = multierror.Append(dv.err, fmt.Errorf("unresolved or incorrect map field type %q", f.GetName()))
				return
			}

			if mapType.Field[0].GetType() != descriptor.FieldDescriptorProto_TYPE_STRING ||
				mapType.Field[1].GetType() != descriptor.FieldDescriptorProto_TYPE_STRING {
				dv.err = multierror.Append(dv.err, errors.New("only map<string, string> is supported in expressions"))
				return
			}

			// translate map<X, Y> proto3 field type to record Mixer type (map[string]string)
			if _, ok := dv.out[f.GetName()]; !ok {
				dv.out[f.GetName()] = make(map[string]string)
			}

			visitor := &mapVisitor{desc: mapType}
			for len(v) > 0 {
				_, _, m := wire.ConsumeField(visitor, v)
				if m < 0 {
					dv.err = multierror.Append(dv.err, fmt.Errorf("failed to parse map field %q: %v", f.GetName(), wire.ParseError(m)))
					return
				}
				v = v[m:]
			}

			dv.out[f.GetName()].(map[string]string)[visitor.key] = visitor.value
			return
		}
		// TODO(kuat): implement sub-message decoding
		return
	}

	// fallback into packed repeated encoding
	if f.IsRepeated() && (f.IsPacked() || f.IsPacked3()) {
		var m int
		for len(v) > 0 {
			switch f.GetType() {
			case descriptor.FieldDescriptorProto_TYPE_BOOL,
				descriptor.FieldDescriptorProto_TYPE_INT32,
				descriptor.FieldDescriptorProto_TYPE_INT64,
				descriptor.FieldDescriptorProto_TYPE_UINT32,
				descriptor.FieldDescriptorProto_TYPE_UINT64,
				descriptor.FieldDescriptorProto_TYPE_SINT32,
				descriptor.FieldDescriptorProto_TYPE_SINT64,
				descriptor.FieldDescriptorProto_TYPE_ENUM:
				var elt uint64
				elt, m = wire.ConsumeVarint(v)
				if m < 0 {
					dv.err = multierror.Append(dv.err, fmt.Errorf("failed to parse packed varint field %q", f.GetName()))
					return
				}
				dv.Varint(n, elt)

			case descriptor.FieldDescriptorProto_TYPE_FIXED32,
				descriptor.FieldDescriptorProto_TYPE_SFIXED32,
				descriptor.FieldDescriptorProto_TYPE_FLOAT:
				var elt uint32
				elt, m = wire.ConsumeFixed32(v)
				if m < 0 {
					dv.err = multierror.Append(dv.err, fmt.Errorf("failed to parse packed fixed32 field %q", f.GetName()))
					return
				}
				dv.Fixed32(n, elt)

			case descriptor.FieldDescriptorProto_TYPE_FIXED64,
				descriptor.FieldDescriptorProto_TYPE_SFIXED64,
				descriptor.FieldDescriptorProto_TYPE_DOUBLE:
				var elt uint64
				elt, m = wire.ConsumeFixed64(v)
				if m < 0 {
					dv.err = multierror.Append(dv.err, fmt.Errorf("failed to parse packed fixed64 field %q", f.GetName()))
					return
				}
				dv.Fixed64(n, elt)

			default:
				dv.err = multierror.Append(dv.err, fmt.Errorf("unexpected field type %q for packed repeated bytes encoding", f.GetType()))
				return
			}
			v = v[m:]
		}
		return
	}

	dv.err = multierror.Append(dv.err, fmt.Errorf("unexpected field type %q for bytes encoding", f.GetType()))
}
