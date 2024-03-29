// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: temporal/api/cloud/cloudservice/v1/service.proto

package cloudservicev1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	CloudService_GetUsers_FullMethodName                    = "/temporal.api.cloud.cloudservice.v1.CloudService/GetUsers"
	CloudService_GetUser_FullMethodName                     = "/temporal.api.cloud.cloudservice.v1.CloudService/GetUser"
	CloudService_CreateUser_FullMethodName                  = "/temporal.api.cloud.cloudservice.v1.CloudService/CreateUser"
	CloudService_UpdateUser_FullMethodName                  = "/temporal.api.cloud.cloudservice.v1.CloudService/UpdateUser"
	CloudService_DeleteUser_FullMethodName                  = "/temporal.api.cloud.cloudservice.v1.CloudService/DeleteUser"
	CloudService_SetUserNamespaceAccess_FullMethodName      = "/temporal.api.cloud.cloudservice.v1.CloudService/SetUserNamespaceAccess"
	CloudService_GetAsyncOperation_FullMethodName           = "/temporal.api.cloud.cloudservice.v1.CloudService/GetAsyncOperation"
	CloudService_CreateNamespace_FullMethodName             = "/temporal.api.cloud.cloudservice.v1.CloudService/CreateNamespace"
	CloudService_GetNamespaces_FullMethodName               = "/temporal.api.cloud.cloudservice.v1.CloudService/GetNamespaces"
	CloudService_GetNamespace_FullMethodName                = "/temporal.api.cloud.cloudservice.v1.CloudService/GetNamespace"
	CloudService_UpdateNamespace_FullMethodName             = "/temporal.api.cloud.cloudservice.v1.CloudService/UpdateNamespace"
	CloudService_RenameCustomSearchAttribute_FullMethodName = "/temporal.api.cloud.cloudservice.v1.CloudService/RenameCustomSearchAttribute"
	CloudService_DeleteNamespace_FullMethodName             = "/temporal.api.cloud.cloudservice.v1.CloudService/DeleteNamespace"
	CloudService_GetRegions_FullMethodName                  = "/temporal.api.cloud.cloudservice.v1.CloudService/GetRegions"
	CloudService_GetRegion_FullMethodName                   = "/temporal.api.cloud.cloudservice.v1.CloudService/GetRegion"
)

// CloudServiceClient is the client API for CloudService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type CloudServiceClient interface {
	// Gets all known users
	GetUsers(ctx context.Context, in *GetUsersRequest, opts ...grpc.CallOption) (*GetUsersResponse, error)
	// Get a user
	GetUser(ctx context.Context, in *GetUserRequest, opts ...grpc.CallOption) (*GetUserResponse, error)
	// Create a user
	CreateUser(ctx context.Context, in *CreateUserRequest, opts ...grpc.CallOption) (*CreateUserResponse, error)
	// Update a user
	UpdateUser(ctx context.Context, in *UpdateUserRequest, opts ...grpc.CallOption) (*UpdateUserResponse, error)
	// Delete a user
	DeleteUser(ctx context.Context, in *DeleteUserRequest, opts ...grpc.CallOption) (*DeleteUserResponse, error)
	// Set a user's access to a namespace
	SetUserNamespaceAccess(ctx context.Context, in *SetUserNamespaceAccessRequest, opts ...grpc.CallOption) (*SetUserNamespaceAccessResponse, error)
	// Get the latest information on an async operation
	GetAsyncOperation(ctx context.Context, in *GetAsyncOperationRequest, opts ...grpc.CallOption) (*GetAsyncOperationResponse, error)
	// Create a new namespace
	CreateNamespace(ctx context.Context, in *CreateNamespaceRequest, opts ...grpc.CallOption) (*CreateNamespaceResponse, error)
	// Get all namespaces
	GetNamespaces(ctx context.Context, in *GetNamespacesRequest, opts ...grpc.CallOption) (*GetNamespacesResponse, error)
	// Get a namespace
	GetNamespace(ctx context.Context, in *GetNamespaceRequest, opts ...grpc.CallOption) (*GetNamespaceResponse, error)
	// Update a namespace
	UpdateNamespace(ctx context.Context, in *UpdateNamespaceRequest, opts ...grpc.CallOption) (*UpdateNamespaceResponse, error)
	// Rename an existing customer search attribute
	RenameCustomSearchAttribute(ctx context.Context, in *RenameCustomSearchAttributeRequest, opts ...grpc.CallOption) (*RenameCustomSearchAttributeResponse, error)
	// Delete a namespace
	DeleteNamespace(ctx context.Context, in *DeleteNamespaceRequest, opts ...grpc.CallOption) (*DeleteNamespaceResponse, error)
	// Get all regions
	GetRegions(ctx context.Context, in *GetRegionsRequest, opts ...grpc.CallOption) (*GetRegionsResponse, error)
	// Get a region
	GetRegion(ctx context.Context, in *GetRegionRequest, opts ...grpc.CallOption) (*GetRegionResponse, error)
}

type cloudServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewCloudServiceClient(cc grpc.ClientConnInterface) CloudServiceClient {
	return &cloudServiceClient{cc}
}

func (c *cloudServiceClient) GetUsers(ctx context.Context, in *GetUsersRequest, opts ...grpc.CallOption) (*GetUsersResponse, error) {
	out := new(GetUsersResponse)
	err := c.cc.Invoke(ctx, CloudService_GetUsers_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) GetUser(ctx context.Context, in *GetUserRequest, opts ...grpc.CallOption) (*GetUserResponse, error) {
	out := new(GetUserResponse)
	err := c.cc.Invoke(ctx, CloudService_GetUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) CreateUser(ctx context.Context, in *CreateUserRequest, opts ...grpc.CallOption) (*CreateUserResponse, error) {
	out := new(CreateUserResponse)
	err := c.cc.Invoke(ctx, CloudService_CreateUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) UpdateUser(ctx context.Context, in *UpdateUserRequest, opts ...grpc.CallOption) (*UpdateUserResponse, error) {
	out := new(UpdateUserResponse)
	err := c.cc.Invoke(ctx, CloudService_UpdateUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) DeleteUser(ctx context.Context, in *DeleteUserRequest, opts ...grpc.CallOption) (*DeleteUserResponse, error) {
	out := new(DeleteUserResponse)
	err := c.cc.Invoke(ctx, CloudService_DeleteUser_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) SetUserNamespaceAccess(ctx context.Context, in *SetUserNamespaceAccessRequest, opts ...grpc.CallOption) (*SetUserNamespaceAccessResponse, error) {
	out := new(SetUserNamespaceAccessResponse)
	err := c.cc.Invoke(ctx, CloudService_SetUserNamespaceAccess_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) GetAsyncOperation(ctx context.Context, in *GetAsyncOperationRequest, opts ...grpc.CallOption) (*GetAsyncOperationResponse, error) {
	out := new(GetAsyncOperationResponse)
	err := c.cc.Invoke(ctx, CloudService_GetAsyncOperation_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) CreateNamespace(ctx context.Context, in *CreateNamespaceRequest, opts ...grpc.CallOption) (*CreateNamespaceResponse, error) {
	out := new(CreateNamespaceResponse)
	err := c.cc.Invoke(ctx, CloudService_CreateNamespace_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) GetNamespaces(ctx context.Context, in *GetNamespacesRequest, opts ...grpc.CallOption) (*GetNamespacesResponse, error) {
	out := new(GetNamespacesResponse)
	err := c.cc.Invoke(ctx, CloudService_GetNamespaces_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) GetNamespace(ctx context.Context, in *GetNamespaceRequest, opts ...grpc.CallOption) (*GetNamespaceResponse, error) {
	out := new(GetNamespaceResponse)
	err := c.cc.Invoke(ctx, CloudService_GetNamespace_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) UpdateNamespace(ctx context.Context, in *UpdateNamespaceRequest, opts ...grpc.CallOption) (*UpdateNamespaceResponse, error) {
	out := new(UpdateNamespaceResponse)
	err := c.cc.Invoke(ctx, CloudService_UpdateNamespace_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) RenameCustomSearchAttribute(ctx context.Context, in *RenameCustomSearchAttributeRequest, opts ...grpc.CallOption) (*RenameCustomSearchAttributeResponse, error) {
	out := new(RenameCustomSearchAttributeResponse)
	err := c.cc.Invoke(ctx, CloudService_RenameCustomSearchAttribute_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) DeleteNamespace(ctx context.Context, in *DeleteNamespaceRequest, opts ...grpc.CallOption) (*DeleteNamespaceResponse, error) {
	out := new(DeleteNamespaceResponse)
	err := c.cc.Invoke(ctx, CloudService_DeleteNamespace_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) GetRegions(ctx context.Context, in *GetRegionsRequest, opts ...grpc.CallOption) (*GetRegionsResponse, error) {
	out := new(GetRegionsResponse)
	err := c.cc.Invoke(ctx, CloudService_GetRegions_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *cloudServiceClient) GetRegion(ctx context.Context, in *GetRegionRequest, opts ...grpc.CallOption) (*GetRegionResponse, error) {
	out := new(GetRegionResponse)
	err := c.cc.Invoke(ctx, CloudService_GetRegion_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CloudServiceServer is the server API for CloudService service.
// All implementations must embed UnimplementedCloudServiceServer
// for forward compatibility
type CloudServiceServer interface {
	// Gets all known users
	GetUsers(context.Context, *GetUsersRequest) (*GetUsersResponse, error)
	// Get a user
	GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error)
	// Create a user
	CreateUser(context.Context, *CreateUserRequest) (*CreateUserResponse, error)
	// Update a user
	UpdateUser(context.Context, *UpdateUserRequest) (*UpdateUserResponse, error)
	// Delete a user
	DeleteUser(context.Context, *DeleteUserRequest) (*DeleteUserResponse, error)
	// Set a user's access to a namespace
	SetUserNamespaceAccess(context.Context, *SetUserNamespaceAccessRequest) (*SetUserNamespaceAccessResponse, error)
	// Get the latest information on an async operation
	GetAsyncOperation(context.Context, *GetAsyncOperationRequest) (*GetAsyncOperationResponse, error)
	// Create a new namespace
	CreateNamespace(context.Context, *CreateNamespaceRequest) (*CreateNamespaceResponse, error)
	// Get all namespaces
	GetNamespaces(context.Context, *GetNamespacesRequest) (*GetNamespacesResponse, error)
	// Get a namespace
	GetNamespace(context.Context, *GetNamespaceRequest) (*GetNamespaceResponse, error)
	// Update a namespace
	UpdateNamespace(context.Context, *UpdateNamespaceRequest) (*UpdateNamespaceResponse, error)
	// Rename an existing customer search attribute
	RenameCustomSearchAttribute(context.Context, *RenameCustomSearchAttributeRequest) (*RenameCustomSearchAttributeResponse, error)
	// Delete a namespace
	DeleteNamespace(context.Context, *DeleteNamespaceRequest) (*DeleteNamespaceResponse, error)
	// Get all regions
	GetRegions(context.Context, *GetRegionsRequest) (*GetRegionsResponse, error)
	// Get a region
	GetRegion(context.Context, *GetRegionRequest) (*GetRegionResponse, error)
	mustEmbedUnimplementedCloudServiceServer()
}

// UnimplementedCloudServiceServer must be embedded to have forward compatible implementations.
type UnimplementedCloudServiceServer struct {
}

func (UnimplementedCloudServiceServer) GetUsers(context.Context, *GetUsersRequest) (*GetUsersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUsers not implemented")
}
func (UnimplementedCloudServiceServer) GetUser(context.Context, *GetUserRequest) (*GetUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUser not implemented")
}
func (UnimplementedCloudServiceServer) CreateUser(context.Context, *CreateUserRequest) (*CreateUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateUser not implemented")
}
func (UnimplementedCloudServiceServer) UpdateUser(context.Context, *UpdateUserRequest) (*UpdateUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateUser not implemented")
}
func (UnimplementedCloudServiceServer) DeleteUser(context.Context, *DeleteUserRequest) (*DeleteUserResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteUser not implemented")
}
func (UnimplementedCloudServiceServer) SetUserNamespaceAccess(context.Context, *SetUserNamespaceAccessRequest) (*SetUserNamespaceAccessResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUserNamespaceAccess not implemented")
}
func (UnimplementedCloudServiceServer) GetAsyncOperation(context.Context, *GetAsyncOperationRequest) (*GetAsyncOperationResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetAsyncOperation not implemented")
}
func (UnimplementedCloudServiceServer) CreateNamespace(context.Context, *CreateNamespaceRequest) (*CreateNamespaceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateNamespace not implemented")
}
func (UnimplementedCloudServiceServer) GetNamespaces(context.Context, *GetNamespacesRequest) (*GetNamespacesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNamespaces not implemented")
}
func (UnimplementedCloudServiceServer) GetNamespace(context.Context, *GetNamespaceRequest) (*GetNamespaceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNamespace not implemented")
}
func (UnimplementedCloudServiceServer) UpdateNamespace(context.Context, *UpdateNamespaceRequest) (*UpdateNamespaceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateNamespace not implemented")
}
func (UnimplementedCloudServiceServer) RenameCustomSearchAttribute(context.Context, *RenameCustomSearchAttributeRequest) (*RenameCustomSearchAttributeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RenameCustomSearchAttribute not implemented")
}
func (UnimplementedCloudServiceServer) DeleteNamespace(context.Context, *DeleteNamespaceRequest) (*DeleteNamespaceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteNamespace not implemented")
}
func (UnimplementedCloudServiceServer) GetRegions(context.Context, *GetRegionsRequest) (*GetRegionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRegions not implemented")
}
func (UnimplementedCloudServiceServer) GetRegion(context.Context, *GetRegionRequest) (*GetRegionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRegion not implemented")
}
func (UnimplementedCloudServiceServer) mustEmbedUnimplementedCloudServiceServer() {}

// UnsafeCloudServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to CloudServiceServer will
// result in compilation errors.
type UnsafeCloudServiceServer interface {
	mustEmbedUnimplementedCloudServiceServer()
}

func RegisterCloudServiceServer(s grpc.ServiceRegistrar, srv CloudServiceServer) {
	s.RegisterService(&CloudService_ServiceDesc, srv)
}

func _CloudService_GetUsers_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUsersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).GetUsers(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_GetUsers_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).GetUsers(ctx, req.(*GetUsersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_GetUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).GetUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_GetUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).GetUser(ctx, req.(*GetUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_CreateUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).CreateUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_CreateUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).CreateUser(ctx, req.(*CreateUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_UpdateUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).UpdateUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_UpdateUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).UpdateUser(ctx, req.(*UpdateUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_DeleteUser_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteUserRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).DeleteUser(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_DeleteUser_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).DeleteUser(ctx, req.(*DeleteUserRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_SetUserNamespaceAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetUserNamespaceAccessRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).SetUserNamespaceAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_SetUserNamespaceAccess_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).SetUserNamespaceAccess(ctx, req.(*SetUserNamespaceAccessRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_GetAsyncOperation_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetAsyncOperationRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).GetAsyncOperation(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_GetAsyncOperation_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).GetAsyncOperation(ctx, req.(*GetAsyncOperationRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_CreateNamespace_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateNamespaceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).CreateNamespace(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_CreateNamespace_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).CreateNamespace(ctx, req.(*CreateNamespaceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_GetNamespaces_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetNamespacesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).GetNamespaces(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_GetNamespaces_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).GetNamespaces(ctx, req.(*GetNamespacesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_GetNamespace_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetNamespaceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).GetNamespace(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_GetNamespace_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).GetNamespace(ctx, req.(*GetNamespaceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_UpdateNamespace_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateNamespaceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).UpdateNamespace(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_UpdateNamespace_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).UpdateNamespace(ctx, req.(*UpdateNamespaceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_RenameCustomSearchAttribute_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RenameCustomSearchAttributeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).RenameCustomSearchAttribute(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_RenameCustomSearchAttribute_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).RenameCustomSearchAttribute(ctx, req.(*RenameCustomSearchAttributeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_DeleteNamespace_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteNamespaceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).DeleteNamespace(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_DeleteNamespace_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).DeleteNamespace(ctx, req.(*DeleteNamespaceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_GetRegions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRegionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).GetRegions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_GetRegions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).GetRegions(ctx, req.(*GetRegionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _CloudService_GetRegion_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRegionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CloudServiceServer).GetRegion(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: CloudService_GetRegion_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CloudServiceServer).GetRegion(ctx, req.(*GetRegionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// CloudService_ServiceDesc is the grpc.ServiceDesc for CloudService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var CloudService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "temporal.api.cloud.cloudservice.v1.CloudService",
	HandlerType: (*CloudServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetUsers",
			Handler:    _CloudService_GetUsers_Handler,
		},
		{
			MethodName: "GetUser",
			Handler:    _CloudService_GetUser_Handler,
		},
		{
			MethodName: "CreateUser",
			Handler:    _CloudService_CreateUser_Handler,
		},
		{
			MethodName: "UpdateUser",
			Handler:    _CloudService_UpdateUser_Handler,
		},
		{
			MethodName: "DeleteUser",
			Handler:    _CloudService_DeleteUser_Handler,
		},
		{
			MethodName: "SetUserNamespaceAccess",
			Handler:    _CloudService_SetUserNamespaceAccess_Handler,
		},
		{
			MethodName: "GetAsyncOperation",
			Handler:    _CloudService_GetAsyncOperation_Handler,
		},
		{
			MethodName: "CreateNamespace",
			Handler:    _CloudService_CreateNamespace_Handler,
		},
		{
			MethodName: "GetNamespaces",
			Handler:    _CloudService_GetNamespaces_Handler,
		},
		{
			MethodName: "GetNamespace",
			Handler:    _CloudService_GetNamespace_Handler,
		},
		{
			MethodName: "UpdateNamespace",
			Handler:    _CloudService_UpdateNamespace_Handler,
		},
		{
			MethodName: "RenameCustomSearchAttribute",
			Handler:    _CloudService_RenameCustomSearchAttribute_Handler,
		},
		{
			MethodName: "DeleteNamespace",
			Handler:    _CloudService_DeleteNamespace_Handler,
		},
		{
			MethodName: "GetRegions",
			Handler:    _CloudService_GetRegions_Handler,
		},
		{
			MethodName: "GetRegion",
			Handler:    _CloudService_GetRegion_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "temporal/api/cloud/cloudservice/v1/service.proto",
}
