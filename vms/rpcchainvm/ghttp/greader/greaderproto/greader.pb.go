// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.12.4
// source: greader.proto

package greaderproto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ReadRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Length int32 `protobuf:"varint,1,opt,name=length,proto3" json:"length,omitempty"`
}

func (x *ReadRequest) Reset() {
	*x = ReadRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_greader_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReadRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReadRequest) ProtoMessage() {}

func (x *ReadRequest) ProtoReflect() protoreflect.Message {
	mi := &file_greader_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReadRequest.ProtoReflect.Descriptor instead.
func (*ReadRequest) Descriptor() ([]byte, []int) {
	return file_greader_proto_rawDescGZIP(), []int{0}
}

func (x *ReadRequest) GetLength() int32 {
	if x != nil {
		return x.Length
	}
	return 0
}

type ReadResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Read    []byte `protobuf:"bytes,1,opt,name=read,proto3" json:"read,omitempty"`
	Error   string `protobuf:"bytes,2,opt,name=error,proto3" json:"error,omitempty"`
	Errored bool   `protobuf:"varint,3,opt,name=errored,proto3" json:"errored,omitempty"`
}

func (x *ReadResponse) Reset() {
	*x = ReadResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_greader_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ReadResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReadResponse) ProtoMessage() {}

func (x *ReadResponse) ProtoReflect() protoreflect.Message {
	mi := &file_greader_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReadResponse.ProtoReflect.Descriptor instead.
func (*ReadResponse) Descriptor() ([]byte, []int) {
	return file_greader_proto_rawDescGZIP(), []int{1}
}

func (x *ReadResponse) GetRead() []byte {
	if x != nil {
		return x.Read
	}
	return nil
}

func (x *ReadResponse) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

func (x *ReadResponse) GetErrored() bool {
	if x != nil {
		return x.Errored
	}
	return false
}

var File_greader_proto protoreflect.FileDescriptor

var file_greader_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x67, 0x72, 0x65, 0x61, 0x64, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x0c, 0x67, 0x72, 0x65, 0x61, 0x64, 0x65, 0x72, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x25, 0x0a,
	0x0b, 0x52, 0x65, 0x61, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06,
	0x6c, 0x65, 0x6e, 0x67, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x06, 0x6c, 0x65,
	0x6e, 0x67, 0x74, 0x68, 0x22, 0x52, 0x0a, 0x0c, 0x52, 0x65, 0x61, 0x64, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x72, 0x65, 0x61, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0c, 0x52, 0x04, 0x72, 0x65, 0x61, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72, 0x6f,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x18,
	0x0a, 0x07, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x65, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x07, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x65, 0x64, 0x32, 0x47, 0x0a, 0x06, 0x52, 0x65, 0x61, 0x64,
	0x65, 0x72, 0x12, 0x3d, 0x0a, 0x04, 0x52, 0x65, 0x61, 0x64, 0x12, 0x19, 0x2e, 0x67, 0x72, 0x65,
	0x61, 0x64, 0x65, 0x72, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x52, 0x65, 0x61, 0x64, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x67, 0x72, 0x65, 0x61, 0x64, 0x65, 0x72, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x2e, 0x52, 0x65, 0x61, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x42, 0x47, 0x5a, 0x45, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x61, 0x76, 0x61, 0x2d, 0x6c, 0x61, 0x62, 0x73, 0x2f, 0x61, 0x76, 0x61, 0x6c, 0x61, 0x6e, 0x63,
	0x68, 0x65, 0x67, 0x6f, 0x2f, 0x72, 0x70, 0x63, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x76, 0x6d, 0x2f,
	0x67, 0x68, 0x74, 0x74, 0x70, 0x2f, 0x67, 0x72, 0x65, 0x61, 0x64, 0x65, 0x72, 0x2f, 0x67, 0x72,
	0x65, 0x61, 0x64, 0x65, 0x72, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_greader_proto_rawDescOnce sync.Once
	file_greader_proto_rawDescData = file_greader_proto_rawDesc
)

func file_greader_proto_rawDescGZIP() []byte {
	file_greader_proto_rawDescOnce.Do(func() {
		file_greader_proto_rawDescData = protoimpl.X.CompressGZIP(file_greader_proto_rawDescData)
	})
	return file_greader_proto_rawDescData
}

var file_greader_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_greader_proto_goTypes = []interface{}{
	(*ReadRequest)(nil),  // 0: greaderproto.ReadRequest
	(*ReadResponse)(nil), // 1: greaderproto.ReadResponse
}
var file_greader_proto_depIdxs = []int32{
	0, // 0: greaderproto.Reader.Read:input_type -> greaderproto.ReadRequest
	1, // 1: greaderproto.Reader.Read:output_type -> greaderproto.ReadResponse
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_greader_proto_init() }
func file_greader_proto_init() {
	if File_greader_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_greader_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReadRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_greader_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ReadResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_greader_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_greader_proto_goTypes,
		DependencyIndexes: file_greader_proto_depIdxs,
		MessageInfos:      file_greader_proto_msgTypes,
	}.Build()
	File_greader_proto = out.File
	file_greader_proto_rawDesc = nil
	file_greader_proto_goTypes = nil
	file_greader_proto_depIdxs = nil
}
