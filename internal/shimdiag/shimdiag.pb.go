// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.29.0
// 	protoc        v3.19.4
// source: github.com/Microsoft/hcsshim/internal/shimdiag/shimdiag.proto

package shimdiag

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

type ExecProcessRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Args     []string `protobuf:"bytes,1,rep,name=args,proto3" json:"args,omitempty"`
	Workdir  string   `protobuf:"bytes,2,opt,name=workdir,proto3" json:"workdir,omitempty"`
	Terminal bool     `protobuf:"varint,3,opt,name=terminal,proto3" json:"terminal,omitempty"`
	Stdin    string   `protobuf:"bytes,4,opt,name=stdin,proto3" json:"stdin,omitempty"`
	Stdout   string   `protobuf:"bytes,5,opt,name=stdout,proto3" json:"stdout,omitempty"`
	Stderr   string   `protobuf:"bytes,6,opt,name=stderr,proto3" json:"stderr,omitempty"`
}

func (x *ExecProcessRequest) Reset() {
	*x = ExecProcessRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ExecProcessRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecProcessRequest) ProtoMessage() {}

func (x *ExecProcessRequest) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecProcessRequest.ProtoReflect.Descriptor instead.
func (*ExecProcessRequest) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{0}
}

func (x *ExecProcessRequest) GetArgs() []string {
	if x != nil {
		return x.Args
	}
	return nil
}

func (x *ExecProcessRequest) GetWorkdir() string {
	if x != nil {
		return x.Workdir
	}
	return ""
}

func (x *ExecProcessRequest) GetTerminal() bool {
	if x != nil {
		return x.Terminal
	}
	return false
}

func (x *ExecProcessRequest) GetStdin() string {
	if x != nil {
		return x.Stdin
	}
	return ""
}

func (x *ExecProcessRequest) GetStdout() string {
	if x != nil {
		return x.Stdout
	}
	return ""
}

func (x *ExecProcessRequest) GetStderr() string {
	if x != nil {
		return x.Stderr
	}
	return ""
}

type ExecProcessResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ExitCode int32 `protobuf:"varint,1,opt,name=exit_code,json=exitCode,proto3" json:"exit_code,omitempty"`
}

func (x *ExecProcessResponse) Reset() {
	*x = ExecProcessResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ExecProcessResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ExecProcessResponse) ProtoMessage() {}

func (x *ExecProcessResponse) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ExecProcessResponse.ProtoReflect.Descriptor instead.
func (*ExecProcessResponse) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{1}
}

func (x *ExecProcessResponse) GetExitCode() int32 {
	if x != nil {
		return x.ExitCode
	}
	return 0
}

type StacksRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *StacksRequest) Reset() {
	*x = StacksRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StacksRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StacksRequest) ProtoMessage() {}

func (x *StacksRequest) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StacksRequest.ProtoReflect.Descriptor instead.
func (*StacksRequest) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{2}
}

type StacksResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Stacks      string `protobuf:"bytes,1,opt,name=stacks,proto3" json:"stacks,omitempty"`
	GuestStacks string `protobuf:"bytes,2,opt,name=guest_stacks,json=guestStacks,proto3" json:"guest_stacks,omitempty"`
}

func (x *StacksResponse) Reset() {
	*x = StacksResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StacksResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StacksResponse) ProtoMessage() {}

func (x *StacksResponse) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StacksResponse.ProtoReflect.Descriptor instead.
func (*StacksResponse) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{3}
}

func (x *StacksResponse) GetStacks() string {
	if x != nil {
		return x.Stacks
	}
	return ""
}

func (x *StacksResponse) GetGuestStacks() string {
	if x != nil {
		return x.GuestStacks
	}
	return ""
}

type ShareRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	HostPath string `protobuf:"bytes,1,opt,name=host_path,json=hostPath,proto3" json:"host_path,omitempty"`
	UvmPath  string `protobuf:"bytes,2,opt,name=uvm_path,json=uvmPath,proto3" json:"uvm_path,omitempty"`
	ReadOnly bool   `protobuf:"varint,3,opt,name=read_only,json=readOnly,proto3" json:"read_only,omitempty"`
}

func (x *ShareRequest) Reset() {
	*x = ShareRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ShareRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ShareRequest) ProtoMessage() {}

func (x *ShareRequest) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ShareRequest.ProtoReflect.Descriptor instead.
func (*ShareRequest) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{4}
}

func (x *ShareRequest) GetHostPath() string {
	if x != nil {
		return x.HostPath
	}
	return ""
}

func (x *ShareRequest) GetUvmPath() string {
	if x != nil {
		return x.UvmPath
	}
	return ""
}

func (x *ShareRequest) GetReadOnly() bool {
	if x != nil {
		return x.ReadOnly
	}
	return false
}

type ShareResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *ShareResponse) Reset() {
	*x = ShareResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ShareResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ShareResponse) ProtoMessage() {}

func (x *ShareResponse) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ShareResponse.ProtoReflect.Descriptor instead.
func (*ShareResponse) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{5}
}

type PidRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PidRequest) Reset() {
	*x = PidRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PidRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PidRequest) ProtoMessage() {}

func (x *PidRequest) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PidRequest.ProtoReflect.Descriptor instead.
func (*PidRequest) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{6}
}

type PidResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Pid int32 `protobuf:"varint,1,opt,name=pid,proto3" json:"pid,omitempty"`
}

func (x *PidResponse) Reset() {
	*x = PidResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PidResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PidResponse) ProtoMessage() {}

func (x *PidResponse) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PidResponse.ProtoReflect.Descriptor instead.
func (*PidResponse) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{7}
}

func (x *PidResponse) GetPid() int32 {
	if x != nil {
		return x.Pid
	}
	return 0
}

type TasksRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Execs bool `protobuf:"varint,1,opt,name=execs,proto3" json:"execs,omitempty"`
}

func (x *TasksRequest) Reset() {
	*x = TasksRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TasksRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TasksRequest) ProtoMessage() {}

func (x *TasksRequest) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TasksRequest.ProtoReflect.Descriptor instead.
func (*TasksRequest) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{8}
}

func (x *TasksRequest) GetExecs() bool {
	if x != nil {
		return x.Execs
	}
	return false
}

type Task struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID    string  `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Execs []*Exec `protobuf:"bytes,2,rep,name=execs,proto3" json:"execs,omitempty"`
}

func (x *Task) Reset() {
	*x = Task{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Task) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Task) ProtoMessage() {}

func (x *Task) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Task.ProtoReflect.Descriptor instead.
func (*Task) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{9}
}

func (x *Task) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *Task) GetExecs() []*Exec {
	if x != nil {
		return x.Execs
	}
	return nil
}

type Exec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ID    string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	State string `protobuf:"bytes,2,opt,name=state,proto3" json:"state,omitempty"`
}

func (x *Exec) Reset() {
	*x = Exec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Exec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Exec) ProtoMessage() {}

func (x *Exec) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Exec.ProtoReflect.Descriptor instead.
func (*Exec) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{10}
}

func (x *Exec) GetID() string {
	if x != nil {
		return x.ID
	}
	return ""
}

func (x *Exec) GetState() string {
	if x != nil {
		return x.State
	}
	return ""
}

type TasksResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tasks []*Task `protobuf:"bytes,1,rep,name=tasks,proto3" json:"tasks,omitempty"`
}

func (x *TasksResponse) Reset() {
	*x = TasksResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TasksResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TasksResponse) ProtoMessage() {}

func (x *TasksResponse) ProtoReflect() protoreflect.Message {
	mi := &file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TasksResponse.ProtoReflect.Descriptor instead.
func (*TasksResponse) Descriptor() ([]byte, []int) {
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP(), []int{11}
}

func (x *TasksResponse) GetTasks() []*Task {
	if x != nil {
		return x.Tasks
	}
	return nil
}

var File_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto protoreflect.FileDescriptor

var file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDesc = []byte{
	0x0a, 0x3d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4d, 0x69, 0x63,
	0x72, 0x6f, 0x73, 0x6f, 0x66, 0x74, 0x2f, 0x68, 0x63, 0x73, 0x73, 0x68, 0x69, 0x6d, 0x2f, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x73, 0x68, 0x69, 0x6d, 0x64, 0x69, 0x61, 0x67,
	0x2f, 0x73, 0x68, 0x69, 0x6d, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x19, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e, 0x68,
	0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x22, 0xa4, 0x01, 0x0a, 0x12, 0x45,
	0x78, 0x65, 0x63, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x72, 0x67, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52,
	0x04, 0x61, 0x72, 0x67, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x77, 0x6f, 0x72, 0x6b, 0x64, 0x69, 0x72,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x77, 0x6f, 0x72, 0x6b, 0x64, 0x69, 0x72, 0x12,
	0x1a, 0x0a, 0x08, 0x74, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x61, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x08, 0x74, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x61, 0x6c, 0x12, 0x14, 0x0a, 0x05, 0x73,
	0x74, 0x64, 0x69, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x73, 0x74, 0x64, 0x69,
	0x6e, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x64, 0x6f, 0x75, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x73, 0x74, 0x64, 0x6f, 0x75, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x64,
	0x65, 0x72, 0x72, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x64, 0x65, 0x72,
	0x72, 0x22, 0x32, 0x0a, 0x13, 0x45, 0x78, 0x65, 0x63, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x65, 0x78, 0x69, 0x74,
	0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x65, 0x78, 0x69,
	0x74, 0x43, 0x6f, 0x64, 0x65, 0x22, 0x0f, 0x0a, 0x0d, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x73, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x4b, 0x0a, 0x0e, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x73,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x63,
	0x6b, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x61, 0x63, 0x6b, 0x73,
	0x12, 0x21, 0x0a, 0x0c, 0x67, 0x75, 0x65, 0x73, 0x74, 0x5f, 0x73, 0x74, 0x61, 0x63, 0x6b, 0x73,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x67, 0x75, 0x65, 0x73, 0x74, 0x53, 0x74, 0x61,
	0x63, 0x6b, 0x73, 0x22, 0x63, 0x0a, 0x0c, 0x53, 0x68, 0x61, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x68, 0x6f, 0x73, 0x74, 0x5f, 0x70, 0x61, 0x74, 0x68,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x50, 0x61, 0x74, 0x68,
	0x12, 0x19, 0x0a, 0x08, 0x75, 0x76, 0x6d, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x75, 0x76, 0x6d, 0x50, 0x61, 0x74, 0x68, 0x12, 0x1b, 0x0a, 0x09, 0x72,
	0x65, 0x61, 0x64, 0x5f, 0x6f, 0x6e, 0x6c, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08,
	0x72, 0x65, 0x61, 0x64, 0x4f, 0x6e, 0x6c, 0x79, 0x22, 0x0f, 0x0a, 0x0d, 0x53, 0x68, 0x61, 0x72,
	0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x0c, 0x0a, 0x0a, 0x50, 0x69, 0x64,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x22, 0x1f, 0x0a, 0x0b, 0x50, 0x69, 0x64, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x70, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x05, 0x52, 0x03, 0x70, 0x69, 0x64, 0x22, 0x24, 0x0a, 0x0c, 0x54, 0x61, 0x73, 0x6b,
	0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x78, 0x65, 0x63,
	0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x65, 0x78, 0x65, 0x63, 0x73, 0x22, 0x4d,
	0x0a, 0x04, 0x54, 0x61, 0x73, 0x6b, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x35, 0x0a, 0x05, 0x65, 0x78, 0x65, 0x63, 0x73, 0x18,
	0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65,
	0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61,
	0x67, 0x2e, 0x45, 0x78, 0x65, 0x63, 0x52, 0x05, 0x65, 0x78, 0x65, 0x63, 0x73, 0x22, 0x2c, 0x0a,
	0x04, 0x45, 0x78, 0x65, 0x63, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x22, 0x46, 0x0a, 0x0d, 0x54,
	0x61, 0x73, 0x6b, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x35, 0x0a, 0x05,
	0x74, 0x61, 0x73, 0x6b, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x63, 0x6f,
	0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e,
	0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x05, 0x74, 0x61,
	0x73, 0x6b, 0x73, 0x32, 0xf8, 0x03, 0x0a, 0x08, 0x53, 0x68, 0x69, 0x6d, 0x44, 0x69, 0x61, 0x67,
	0x12, 0x6f, 0x0a, 0x0e, 0x44, 0x69, 0x61, 0x67, 0x45, 0x78, 0x65, 0x63, 0x49, 0x6e, 0x48, 0x6f,
	0x73, 0x74, 0x12, 0x2d, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e,
	0x72, 0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x45,
	0x78, 0x65, 0x63, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x2e, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72,
	0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x45, 0x78,
	0x65, 0x63, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x61, 0x0a, 0x0a, 0x44, 0x69, 0x61, 0x67, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x73, 0x12,
	0x28, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e,
	0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x53, 0x74, 0x61, 0x63,
	0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x29, 0x2e, 0x63, 0x6f, 0x6e, 0x74,
	0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76, 0x31,
	0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x73, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x5e, 0x0a, 0x09, 0x44, 0x69, 0x61, 0x67, 0x54, 0x61, 0x73, 0x6b,
	0x73, 0x12, 0x27, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72,
	0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x54, 0x61,
	0x73, 0x6b, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x28, 0x2e, 0x63, 0x6f, 0x6e,
	0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76,
	0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x54, 0x61, 0x73, 0x6b, 0x73, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x5e, 0x0a, 0x09, 0x44, 0x69, 0x61, 0x67, 0x53, 0x68, 0x61, 0x72,
	0x65, 0x12, 0x27, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72,
	0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x53, 0x68,
	0x61, 0x72, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x28, 0x2e, 0x63, 0x6f, 0x6e,
	0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76,
	0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x53, 0x68, 0x61, 0x72, 0x65, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x58, 0x0a, 0x07, 0x44, 0x69, 0x61, 0x67, 0x50, 0x69, 0x64, 0x12,
	0x25, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e,
	0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69, 0x61, 0x67, 0x2e, 0x50, 0x69, 0x64, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x26, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e,
	0x65, 0x72, 0x64, 0x2e, 0x72, 0x75, 0x6e, 0x68, 0x63, 0x73, 0x2e, 0x76, 0x31, 0x2e, 0x64, 0x69,
	0x61, 0x67, 0x2e, 0x50, 0x69, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x39,
	0x5a, 0x37, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4d, 0x69, 0x63,
	0x72, 0x6f, 0x73, 0x6f, 0x66, 0x74, 0x2f, 0x68, 0x63, 0x73, 0x73, 0x68, 0x69, 0x6d, 0x2f, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x73, 0x68, 0x69, 0x6d, 0x64, 0x69, 0x61, 0x67,
	0x3b, 0x73, 0x68, 0x69, 0x6d, 0x64, 0x69, 0x61, 0x67, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescOnce sync.Once
	file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescData = file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDesc
)

func file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescGZIP() []byte {
	file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescOnce.Do(func() {
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescData = protoimpl.X.CompressGZIP(file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescData)
	})
	return file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDescData
}

var file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_goTypes = []interface{}{
	(*ExecProcessRequest)(nil),  // 0: containerd.runhcs.v1.diag.ExecProcessRequest
	(*ExecProcessResponse)(nil), // 1: containerd.runhcs.v1.diag.ExecProcessResponse
	(*StacksRequest)(nil),       // 2: containerd.runhcs.v1.diag.StacksRequest
	(*StacksResponse)(nil),      // 3: containerd.runhcs.v1.diag.StacksResponse
	(*ShareRequest)(nil),        // 4: containerd.runhcs.v1.diag.ShareRequest
	(*ShareResponse)(nil),       // 5: containerd.runhcs.v1.diag.ShareResponse
	(*PidRequest)(nil),          // 6: containerd.runhcs.v1.diag.PidRequest
	(*PidResponse)(nil),         // 7: containerd.runhcs.v1.diag.PidResponse
	(*TasksRequest)(nil),        // 8: containerd.runhcs.v1.diag.TasksRequest
	(*Task)(nil),                // 9: containerd.runhcs.v1.diag.Task
	(*Exec)(nil),                // 10: containerd.runhcs.v1.diag.Exec
	(*TasksResponse)(nil),       // 11: containerd.runhcs.v1.diag.TasksResponse
}
var file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_depIdxs = []int32{
	10, // 0: containerd.runhcs.v1.diag.Task.execs:type_name -> containerd.runhcs.v1.diag.Exec
	9,  // 1: containerd.runhcs.v1.diag.TasksResponse.tasks:type_name -> containerd.runhcs.v1.diag.Task
	0,  // 2: containerd.runhcs.v1.diag.ShimDiag.DiagExecInHost:input_type -> containerd.runhcs.v1.diag.ExecProcessRequest
	2,  // 3: containerd.runhcs.v1.diag.ShimDiag.DiagStacks:input_type -> containerd.runhcs.v1.diag.StacksRequest
	8,  // 4: containerd.runhcs.v1.diag.ShimDiag.DiagTasks:input_type -> containerd.runhcs.v1.diag.TasksRequest
	4,  // 5: containerd.runhcs.v1.diag.ShimDiag.DiagShare:input_type -> containerd.runhcs.v1.diag.ShareRequest
	6,  // 6: containerd.runhcs.v1.diag.ShimDiag.DiagPid:input_type -> containerd.runhcs.v1.diag.PidRequest
	1,  // 7: containerd.runhcs.v1.diag.ShimDiag.DiagExecInHost:output_type -> containerd.runhcs.v1.diag.ExecProcessResponse
	3,  // 8: containerd.runhcs.v1.diag.ShimDiag.DiagStacks:output_type -> containerd.runhcs.v1.diag.StacksResponse
	11, // 9: containerd.runhcs.v1.diag.ShimDiag.DiagTasks:output_type -> containerd.runhcs.v1.diag.TasksResponse
	5,  // 10: containerd.runhcs.v1.diag.ShimDiag.DiagShare:output_type -> containerd.runhcs.v1.diag.ShareResponse
	7,  // 11: containerd.runhcs.v1.diag.ShimDiag.DiagPid:output_type -> containerd.runhcs.v1.diag.PidResponse
	7,  // [7:12] is the sub-list for method output_type
	2,  // [2:7] is the sub-list for method input_type
	2,  // [2:2] is the sub-list for extension type_name
	2,  // [2:2] is the sub-list for extension extendee
	0,  // [0:2] is the sub-list for field type_name
}

func init() { file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_init() }
func file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_init() {
	if File_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ExecProcessRequest); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ExecProcessResponse); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StacksRequest); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StacksResponse); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ShareRequest); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ShareResponse); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PidRequest); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PidResponse); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TasksRequest); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Task); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Exec); i {
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
		file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TasksResponse); i {
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
			RawDescriptor: file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_goTypes,
		DependencyIndexes: file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_depIdxs,
		MessageInfos:      file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_msgTypes,
	}.Build()
	File_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto = out.File
	file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_rawDesc = nil
	file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_goTypes = nil
	file_github_com_Microsoft_hcsshim_internal_shimdiag_shimdiag_proto_depIdxs = nil
}
