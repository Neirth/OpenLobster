// Copyright (c) OpenLobster contributors. See LICENSE for details.

package protocol

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	PluginServiceName = "openlobster.plugin.v1.PluginService"

	PluginServiceGetInfoFullMethodName = "/openlobster.plugin.v1.PluginService/GetInfo"
	PluginServiceCallFullMethodName    = "/openlobster.plugin.v1.PluginService/Call"
	PluginServiceCloseFullMethodName   = "/openlobster.plugin.v1.PluginService/Close"
	PluginServiceEventsFullMethodName  = "/openlobster.plugin.v1.PluginService/StreamEvents"
)

const (
	EventTypeEmitMessage = "emit_message"
	EventTypeLog         = "log"
)

type GetInfoRequest struct{}

type GetInfoResponse struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type,omitempty"`
	Schema      []byte   `json:"schema,omitempty"`
	Properties  []byte   `json:"properties,omitempty"`
	Exports     []string `json:"exports,omitempty"`
}

type CallRequest struct {
	Function string `json:"function,omitempty"`
	Input    []byte `json:"input,omitempty"`
}

type CallResponse struct {
	Output []byte `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}

type StreamEventsRequest struct{}

type Event struct {
	Type    string `json:"type,omitempty"`
	Payload []byte `json:"payload,omitempty"`
	Level   string `json:"level,omitempty"`
	Message string `json:"message,omitempty"`
}

type CloseRequest struct{}

type CloseResponse struct{}

type PluginServiceClient interface {
	GetInfo(ctx context.Context, in *GetInfoRequest, opts ...grpc.CallOption) (*GetInfoResponse, error)
	Call(ctx context.Context, in *CallRequest, opts ...grpc.CallOption) (*CallResponse, error)
	StreamEvents(ctx context.Context, in *StreamEventsRequest, opts ...grpc.CallOption) (PluginService_StreamEventsClient, error)
	Close(ctx context.Context, in *CloseRequest, opts ...grpc.CallOption) (*CloseResponse, error)
}

type pluginServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPluginServiceClient(cc grpc.ClientConnInterface) PluginServiceClient {
	return &pluginServiceClient{cc: cc}
}

func (c *pluginServiceClient) GetInfo(ctx context.Context, in *GetInfoRequest, opts ...grpc.CallOption) (*GetInfoResponse, error) {
	out := new(GetInfoResponse)
	err := c.cc.Invoke(ctx, PluginServiceGetInfoFullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pluginServiceClient) Call(ctx context.Context, in *CallRequest, opts ...grpc.CallOption) (*CallResponse, error) {
	out := new(CallResponse)
	err := c.cc.Invoke(ctx, PluginServiceCallFullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pluginServiceClient) StreamEvents(ctx context.Context, in *StreamEventsRequest, opts ...grpc.CallOption) (PluginService_StreamEventsClient, error) {
	stream, err := c.cc.NewStream(ctx, &PluginService_ServiceDesc.Streams[0], PluginServiceEventsFullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &pluginServiceStreamEventsClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

func (c *pluginServiceClient) Close(ctx context.Context, in *CloseRequest, opts ...grpc.CallOption) (*CloseResponse, error) {
	out := new(CloseResponse)
	err := c.cc.Invoke(ctx, PluginServiceCloseFullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type PluginService_StreamEventsClient interface {
	Recv() (*Event, error)
	grpc.ClientStream
}

type pluginServiceStreamEventsClient struct {
	grpc.ClientStream
}

func (x *pluginServiceStreamEventsClient) Recv() (*Event, error) {
	m := new(Event)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type PluginServiceServer interface {
	GetInfo(context.Context, *GetInfoRequest) (*GetInfoResponse, error)
	Call(context.Context, *CallRequest) (*CallResponse, error)
	StreamEvents(*StreamEventsRequest, PluginService_StreamEventsServer) error
	Close(context.Context, *CloseRequest) (*CloseResponse, error)
}

// UnimplementedPluginServiceServer can be embedded to have forward compatible implementations.
type UnimplementedPluginServiceServer struct{}

func (UnimplementedPluginServiceServer) GetInfo(context.Context, *GetInfoRequest) (*GetInfoResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetInfo not implemented")
}

func (UnimplementedPluginServiceServer) Call(context.Context, *CallRequest) (*CallResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Call not implemented")
}

func (UnimplementedPluginServiceServer) StreamEvents(*StreamEventsRequest, PluginService_StreamEventsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamEvents not implemented")
}

func (UnimplementedPluginServiceServer) Close(context.Context, *CloseRequest) (*CloseResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Close not implemented")
}

func RegisterPluginServiceServer(s grpc.ServiceRegistrar, srv PluginServiceServer) {
	s.RegisterService(&PluginService_ServiceDesc, srv)
}

type PluginService_StreamEventsServer interface {
	Send(*Event) error
	grpc.ServerStream
}

type pluginServiceStreamEventsServer struct {
	grpc.ServerStream
}

func (x *pluginServiceStreamEventsServer) Send(m *Event) error {
	return x.ServerStream.SendMsg(m)
}

func _PluginService_GetInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetInfoRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServiceServer).GetInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PluginServiceGetInfoFullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginServiceServer).GetInfo(ctx, req.(*GetInfoRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PluginService_Call_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CallRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServiceServer).Call(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PluginServiceCallFullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginServiceServer).Call(ctx, req.(*CallRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PluginService_Close_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CloseRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PluginServiceServer).Close(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PluginServiceCloseFullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PluginServiceServer).Close(ctx, req.(*CloseRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PluginService_StreamEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StreamEventsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(PluginServiceServer).StreamEvents(m, &pluginServiceStreamEventsServer{ServerStream: stream})
}

var PluginService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: PluginServiceName,
	HandlerType: (*PluginServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetInfo",
			Handler:    _PluginService_GetInfo_Handler,
		},
		{
			MethodName: "Call",
			Handler:    _PluginService_Call_Handler,
		},
		{
			MethodName: "Close",
			Handler:    _PluginService_Close_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamEvents",
			Handler:       _PluginService_StreamEvents_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "protocol",
}
