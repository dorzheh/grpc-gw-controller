// Code generated by protoc-gen-go. DO NOT EDIT.
// source: apphcmanager.proto

package apphcmanager

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "google.golang.org/genproto/googleapis/api/annotations"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type GetApphcVersionRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetApphcVersionRequest) Reset()         { *m = GetApphcVersionRequest{} }
func (m *GetApphcVersionRequest) String() string { return proto.CompactTextString(m) }
func (*GetApphcVersionRequest) ProtoMessage()    {}
func (*GetApphcVersionRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_apphcmanager_98ad936e771db2d4, []int{0}
}
func (m *GetApphcVersionRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetApphcVersionRequest.Unmarshal(m, b)
}
func (m *GetApphcVersionRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetApphcVersionRequest.Marshal(b, m, deterministic)
}
func (dst *GetApphcVersionRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetApphcVersionRequest.Merge(dst, src)
}
func (m *GetApphcVersionRequest) XXX_Size() int {
	return xxx_messageInfo_GetApphcVersionRequest.Size(m)
}
func (m *GetApphcVersionRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetApphcVersionRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetApphcVersionRequest proto.InternalMessageInfo

type GetApphcVersionResponse struct {
	Version              string   `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	ApiVersion           string   `protobuf:"bytes,2,opt,name=api_version,json=apiVersion,proto3" json:"api_version,omitempty"`
	GitCommit            string   `protobuf:"bytes,3,opt,name=git_commit,json=gitCommit,proto3" json:"git_commit,omitempty"`
	GitState             string   `protobuf:"bytes,4,opt,name=git_state,json=gitState,proto3" json:"git_state,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetApphcVersionResponse) Reset()         { *m = GetApphcVersionResponse{} }
func (m *GetApphcVersionResponse) String() string { return proto.CompactTextString(m) }
func (*GetApphcVersionResponse) ProtoMessage()    {}
func (*GetApphcVersionResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_apphcmanager_98ad936e771db2d4, []int{1}
}
func (m *GetApphcVersionResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetApphcVersionResponse.Unmarshal(m, b)
}
func (m *GetApphcVersionResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetApphcVersionResponse.Marshal(b, m, deterministic)
}
func (dst *GetApphcVersionResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetApphcVersionResponse.Merge(dst, src)
}
func (m *GetApphcVersionResponse) XXX_Size() int {
	return xxx_messageInfo_GetApphcVersionResponse.Size(m)
}
func (m *GetApphcVersionResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetApphcVersionResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetApphcVersionResponse proto.InternalMessageInfo

func (m *GetApphcVersionResponse) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func (m *GetApphcVersionResponse) GetApiVersion() string {
	if m != nil {
		return m.ApiVersion
	}
	return ""
}

func (m *GetApphcVersionResponse) GetGitCommit() string {
	if m != nil {
		return m.GitCommit
	}
	return ""
}

func (m *GetApphcVersionResponse) GetGitState() string {
	if m != nil {
		return m.GitState
	}
	return ""
}

func init() {
	proto.RegisterType((*GetApphcVersionRequest)(nil), "com.cisco.son.apphcd.api.v1.apphcmanager.GetApphcVersionRequest")
	proto.RegisterType((*GetApphcVersionResponse)(nil), "com.cisco.son.apphcd.api.v1.apphcmanager.GetApphcVersionResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ApphcManagerClient is the client API for ApphcManager service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ApphcManagerClient interface {
	// Obtain Controller version
	GetVersion(ctx context.Context, in *GetApphcVersionRequest, opts ...grpc.CallOption) (*GetApphcVersionResponse, error)
}

type apphcManagerClient struct {
	cc *grpc.ClientConn
}

func NewApphcManagerClient(cc *grpc.ClientConn) ApphcManagerClient {
	return &apphcManagerClient{cc}
}

func (c *apphcManagerClient) GetVersion(ctx context.Context, in *GetApphcVersionRequest, opts ...grpc.CallOption) (*GetApphcVersionResponse, error) {
	out := new(GetApphcVersionResponse)
	err := c.cc.Invoke(ctx, "/com.cisco.son.apphcd.api.v1.apphcmanager.ApphcManager/GetVersion", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ApphcManagerServer is the server API for ApphcManager service.
type ApphcManagerServer interface {
	// Obtain Controller version
	GetVersion(context.Context, *GetApphcVersionRequest) (*GetApphcVersionResponse, error)
}

func RegisterApphcManagerServer(s *grpc.Server, srv ApphcManagerServer) {
	s.RegisterService(&_ApphcManager_serviceDesc, srv)
}

func _ApphcManager_GetVersion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetApphcVersionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApphcManagerServer).GetVersion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/com.cisco.son.apphcd.api.v1.apphcmanager.ApphcManager/GetVersion",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApphcManagerServer).GetVersion(ctx, req.(*GetApphcVersionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _ApphcManager_serviceDesc = grpc.ServiceDesc{
	ServiceName: "com.cisco.son.apphcd.api.v1.apphcmanager.ApphcManager",
	HandlerType: (*ApphcManagerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetVersion",
			Handler:    _ApphcManager_GetVersion_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "apphcmanager.proto",
}

func init() { proto.RegisterFile("apphcmanager.proto", fileDescriptor_apphcmanager_98ad936e771db2d4) }

var fileDescriptor_apphcmanager_98ad936e771db2d4 = []byte{
	// 282 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xa4, 0x91, 0xc1, 0x4a, 0xc3, 0x40,
	0x10, 0x86, 0x49, 0x15, 0xb5, 0x63, 0xf1, 0xb0, 0xa0, 0x0d, 0xd5, 0xa2, 0xf4, 0xd4, 0xd3, 0x86,
	0xea, 0x0b, 0x58, 0x3d, 0xf4, 0xe4, 0xa5, 0x82, 0x07, 0x2f, 0x61, 0x8c, 0xcb, 0x3a, 0x60, 0x76,
	0xd6, 0xec, 0x98, 0x07, 0xf0, 0x09, 0x04, 0x5f, 0xc4, 0x57, 0xf0, 0x19, 0x7c, 0x05, 0x1f, 0x44,
	0xb2, 0x69, 0xa0, 0xa0, 0x07, 0xc1, 0xe3, 0xcc, 0x37, 0xf3, 0xb1, 0xf3, 0x2f, 0x28, 0xf4, 0xfe,
	0xa1, 0x28, 0xd1, 0xa1, 0x35, 0x95, 0xf6, 0x15, 0x0b, 0xab, 0x69, 0xc1, 0xa5, 0x2e, 0x28, 0x14,
	0xac, 0x03, 0x3b, 0x1d, 0x27, 0xee, 0x35, 0x7a, 0xd2, 0xf5, 0x4c, 0xaf, 0xcf, 0x8f, 0x8e, 0x2c,
	0xb3, 0x7d, 0x34, 0x19, 0x7a, 0xca, 0xd0, 0x39, 0x16, 0x14, 0x62, 0x17, 0x5a, 0xcf, 0x24, 0x85,
	0x83, 0x85, 0x91, 0x79, 0xb3, 0x70, 0x63, 0xaa, 0x40, 0xec, 0x96, 0xe6, 0xe9, 0xd9, 0x04, 0x99,
	0xbc, 0x26, 0x30, 0xfc, 0x81, 0x82, 0x67, 0x17, 0x8c, 0x4a, 0x61, 0xbb, 0x6e, 0x5b, 0x69, 0x72,
	0x92, 0x4c, 0xfb, 0xcb, 0xae, 0x54, 0xc7, 0xb0, 0x8b, 0x9e, 0xf2, 0x8e, 0xf6, 0x22, 0x05, 0xf4,
	0xb4, 0x52, 0xa8, 0x31, 0x80, 0x25, 0xc9, 0x0b, 0x2e, 0x4b, 0x92, 0x74, 0x23, 0xf2, 0xbe, 0x25,
	0xb9, 0x8c, 0x0d, 0x75, 0x08, 0x4d, 0x91, 0x07, 0x41, 0x31, 0xe9, 0x66, 0xa4, 0x3b, 0x96, 0xe4,
	0xba, 0xa9, 0x4f, 0x3f, 0x12, 0x18, 0xc4, 0xf7, 0x5c, 0xb5, 0xb7, 0xa9, 0xf7, 0x04, 0x60, 0x61,
	0xa4, 0x73, 0x9f, 0xeb, 0xbf, 0xa6, 0xa2, 0x7f, 0x3f, 0x7a, 0x34, 0xff, 0x87, 0xa1, 0xcd, 0x66,
	0x32, 0x7e, 0xf9, 0xfc, 0x7a, 0xeb, 0x0d, 0xd5, 0x7e, 0x4c, 0xbc, 0x9e, 0x65, 0x71, 0x2b, 0x5b,
	0x25, 0x72, 0xb1, 0x77, 0x3b, 0x58, 0xd7, 0xdc, 0x6d, 0xc5, 0x7f, 0x38, 0xfb, 0x0e, 0x00, 0x00,
	0xff, 0xff, 0xdf, 0xd3, 0xa4, 0xa4, 0xe5, 0x01, 0x00, 0x00,
}
