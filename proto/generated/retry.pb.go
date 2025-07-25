// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v5.29.3
// source: proto/retry.proto

package generated

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RetryState int32

const (
	RetryState_RETRY_STATE_UNSPECIFIED RetryState = 0
	RetryState_RETRY_STATE_SCHEDULED   RetryState = 1
	RetryState_RETRY_STATE_IN_PROGRESS RetryState = 2
	RetryState_RETRY_STATE_COMPLETED   RetryState = 3
	RetryState_RETRY_STATE_FAILED      RetryState = 4
	RetryState_RETRY_STATE_CANCELLED   RetryState = 5
	RetryState_RETRY_STATE_EXPIRED     RetryState = 6
)

// Enum value maps for RetryState.
var (
	RetryState_name = map[int32]string{
		0: "RETRY_STATE_UNSPECIFIED",
		1: "RETRY_STATE_SCHEDULED",
		2: "RETRY_STATE_IN_PROGRESS",
		3: "RETRY_STATE_COMPLETED",
		4: "RETRY_STATE_FAILED",
		5: "RETRY_STATE_CANCELLED",
		6: "RETRY_STATE_EXPIRED",
	}
	RetryState_value = map[string]int32{
		"RETRY_STATE_UNSPECIFIED": 0,
		"RETRY_STATE_SCHEDULED":   1,
		"RETRY_STATE_IN_PROGRESS": 2,
		"RETRY_STATE_COMPLETED":   3,
		"RETRY_STATE_FAILED":      4,
		"RETRY_STATE_CANCELLED":   5,
		"RETRY_STATE_EXPIRED":     6,
	}
)

func (x RetryState) Enum() *RetryState {
	p := new(RetryState)
	*p = x
	return p
}

func (x RetryState) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (RetryState) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_retry_proto_enumTypes[0].Descriptor()
}

func (RetryState) Type() protoreflect.EnumType {
	return &file_proto_retry_proto_enumTypes[0]
}

func (x RetryState) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use RetryState.Descriptor instead.
func (RetryState) EnumDescriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{0}
}

// Request/Response Messages
type ScheduleRetryRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	EventId       string                 `protobuf:"bytes,1,opt,name=event_id,json=eventId,proto3" json:"event_id,omitempty"`
	WebhookId     string                 `protobuf:"bytes,2,opt,name=webhook_id,json=webhookId,proto3" json:"webhook_id,omitempty"`
	CustomerId    string                 `protobuf:"bytes,3,opt,name=customer_id,json=customerId,proto3" json:"customer_id,omitempty"`
	Payload       []byte                 `protobuf:"bytes,4,opt,name=payload,proto3" json:"payload,omitempty"`
	AttemptNumber int32                  `protobuf:"varint,5,opt,name=attempt_number,json=attemptNumber,proto3" json:"attempt_number,omitempty"`
	FailureReason string                 `protobuf:"bytes,6,opt,name=failure_reason,json=failureReason,proto3" json:"failure_reason,omitempty"`
	NextRetryAt   *timestamppb.Timestamp `protobuf:"bytes,7,opt,name=next_retry_at,json=nextRetryAt,proto3" json:"next_retry_at,omitempty"`
	Headers       map[string]string      `protobuf:"bytes,8,rep,name=headers,proto3" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ScheduleRetryRequest) Reset() {
	*x = ScheduleRetryRequest{}
	mi := &file_proto_retry_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ScheduleRetryRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ScheduleRetryRequest) ProtoMessage() {}

func (x *ScheduleRetryRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ScheduleRetryRequest.ProtoReflect.Descriptor instead.
func (*ScheduleRetryRequest) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{0}
}

func (x *ScheduleRetryRequest) GetEventId() string {
	if x != nil {
		return x.EventId
	}
	return ""
}

func (x *ScheduleRetryRequest) GetWebhookId() string {
	if x != nil {
		return x.WebhookId
	}
	return ""
}

func (x *ScheduleRetryRequest) GetCustomerId() string {
	if x != nil {
		return x.CustomerId
	}
	return ""
}

func (x *ScheduleRetryRequest) GetPayload() []byte {
	if x != nil {
		return x.Payload
	}
	return nil
}

func (x *ScheduleRetryRequest) GetAttemptNumber() int32 {
	if x != nil {
		return x.AttemptNumber
	}
	return 0
}

func (x *ScheduleRetryRequest) GetFailureReason() string {
	if x != nil {
		return x.FailureReason
	}
	return ""
}

func (x *ScheduleRetryRequest) GetNextRetryAt() *timestamppb.Timestamp {
	if x != nil {
		return x.NextRetryAt
	}
	return nil
}

func (x *ScheduleRetryRequest) GetHeaders() map[string]string {
	if x != nil {
		return x.Headers
	}
	return nil
}

type ScheduleRetryResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RetryId       string                 `protobuf:"bytes,1,opt,name=retry_id,json=retryId,proto3" json:"retry_id,omitempty"`
	ScheduledAt   *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=scheduled_at,json=scheduledAt,proto3" json:"scheduled_at,omitempty"`
	Success       bool                   `protobuf:"varint,3,opt,name=success,proto3" json:"success,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ScheduleRetryResponse) Reset() {
	*x = ScheduleRetryResponse{}
	mi := &file_proto_retry_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ScheduleRetryResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ScheduleRetryResponse) ProtoMessage() {}

func (x *ScheduleRetryResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ScheduleRetryResponse.ProtoReflect.Descriptor instead.
func (*ScheduleRetryResponse) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{1}
}

func (x *ScheduleRetryResponse) GetRetryId() string {
	if x != nil {
		return x.RetryId
	}
	return ""
}

func (x *ScheduleRetryResponse) GetScheduledAt() *timestamppb.Timestamp {
	if x != nil {
		return x.ScheduledAt
	}
	return nil
}

func (x *ScheduleRetryResponse) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

type GetRetryStatusRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RetryId       string                 `protobuf:"bytes,1,opt,name=retry_id,json=retryId,proto3" json:"retry_id,omitempty"`
	EventId       string                 `protobuf:"bytes,2,opt,name=event_id,json=eventId,proto3" json:"event_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetRetryStatusRequest) Reset() {
	*x = GetRetryStatusRequest{}
	mi := &file_proto_retry_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetRetryStatusRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRetryStatusRequest) ProtoMessage() {}

func (x *GetRetryStatusRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRetryStatusRequest.ProtoReflect.Descriptor instead.
func (*GetRetryStatusRequest) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{2}
}

func (x *GetRetryStatusRequest) GetRetryId() string {
	if x != nil {
		return x.RetryId
	}
	return ""
}

func (x *GetRetryStatusRequest) GetEventId() string {
	if x != nil {
		return x.EventId
	}
	return ""
}

type GetRetryStatusResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Status        *RetryStatus           `protobuf:"bytes,1,opt,name=status,proto3" json:"status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetRetryStatusResponse) Reset() {
	*x = GetRetryStatusResponse{}
	mi := &file_proto_retry_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetRetryStatusResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRetryStatusResponse) ProtoMessage() {}

func (x *GetRetryStatusResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRetryStatusResponse.ProtoReflect.Descriptor instead.
func (*GetRetryStatusResponse) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{3}
}

func (x *GetRetryStatusResponse) GetStatus() *RetryStatus {
	if x != nil {
		return x.Status
	}
	return nil
}

type CancelRetryRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	RetryId       string                 `protobuf:"bytes,1,opt,name=retry_id,json=retryId,proto3" json:"retry_id,omitempty"`
	EventId       string                 `protobuf:"bytes,2,opt,name=event_id,json=eventId,proto3" json:"event_id,omitempty"`
	Reason        string                 `protobuf:"bytes,3,opt,name=reason,proto3" json:"reason,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CancelRetryRequest) Reset() {
	*x = CancelRetryRequest{}
	mi := &file_proto_retry_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CancelRetryRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CancelRetryRequest) ProtoMessage() {}

func (x *CancelRetryRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CancelRetryRequest.ProtoReflect.Descriptor instead.
func (*CancelRetryRequest) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{4}
}

func (x *CancelRetryRequest) GetRetryId() string {
	if x != nil {
		return x.RetryId
	}
	return ""
}

func (x *CancelRetryRequest) GetEventId() string {
	if x != nil {
		return x.EventId
	}
	return ""
}

func (x *CancelRetryRequest) GetReason() string {
	if x != nil {
		return x.Reason
	}
	return ""
}

type CancelRetryResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Success       bool                   `protobuf:"varint,1,opt,name=success,proto3" json:"success,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *CancelRetryResponse) Reset() {
	*x = CancelRetryResponse{}
	mi := &file_proto_retry_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *CancelRetryResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CancelRetryResponse) ProtoMessage() {}

func (x *CancelRetryResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CancelRetryResponse.ProtoReflect.Descriptor instead.
func (*CancelRetryResponse) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{5}
}

func (x *CancelRetryResponse) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

type ListRetriesRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	WebhookId     string                 `protobuf:"bytes,1,opt,name=webhook_id,json=webhookId,proto3" json:"webhook_id,omitempty"`
	CustomerId    string                 `protobuf:"bytes,2,opt,name=customer_id,json=customerId,proto3" json:"customer_id,omitempty"`
	Limit         int32                  `protobuf:"varint,3,opt,name=limit,proto3" json:"limit,omitempty"`
	Offset        int32                  `protobuf:"varint,4,opt,name=offset,proto3" json:"offset,omitempty"`
	State         RetryState             `protobuf:"varint,5,opt,name=state,proto3,enum=retry.RetryState" json:"state,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListRetriesRequest) Reset() {
	*x = ListRetriesRequest{}
	mi := &file_proto_retry_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListRetriesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRetriesRequest) ProtoMessage() {}

func (x *ListRetriesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRetriesRequest.ProtoReflect.Descriptor instead.
func (*ListRetriesRequest) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{6}
}

func (x *ListRetriesRequest) GetWebhookId() string {
	if x != nil {
		return x.WebhookId
	}
	return ""
}

func (x *ListRetriesRequest) GetCustomerId() string {
	if x != nil {
		return x.CustomerId
	}
	return ""
}

func (x *ListRetriesRequest) GetLimit() int32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *ListRetriesRequest) GetOffset() int32 {
	if x != nil {
		return x.Offset
	}
	return 0
}

func (x *ListRetriesRequest) GetState() RetryState {
	if x != nil {
		return x.State
	}
	return RetryState_RETRY_STATE_UNSPECIFIED
}

type ListRetriesResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Retries       []*RetryStatus         `protobuf:"bytes,1,rep,name=retries,proto3" json:"retries,omitempty"`
	TotalCount    int32                  `protobuf:"varint,2,opt,name=total_count,json=totalCount,proto3" json:"total_count,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ListRetriesResponse) Reset() {
	*x = ListRetriesResponse{}
	mi := &file_proto_retry_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ListRetriesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListRetriesResponse) ProtoMessage() {}

func (x *ListRetriesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListRetriesResponse.ProtoReflect.Descriptor instead.
func (*ListRetriesResponse) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{7}
}

func (x *ListRetriesResponse) GetRetries() []*RetryStatus {
	if x != nil {
		return x.Retries
	}
	return nil
}

func (x *ListRetriesResponse) GetTotalCount() int32 {
	if x != nil {
		return x.TotalCount
	}
	return 0
}

type ProcessRetriesRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	BatchSize     int32                  `protobuf:"varint,1,opt,name=batch_size,json=batchSize,proto3" json:"batch_size,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ProcessRetriesRequest) Reset() {
	*x = ProcessRetriesRequest{}
	mi := &file_proto_retry_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProcessRetriesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProcessRetriesRequest) ProtoMessage() {}

func (x *ProcessRetriesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProcessRetriesRequest.ProtoReflect.Descriptor instead.
func (*ProcessRetriesRequest) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{8}
}

func (x *ProcessRetriesRequest) GetBatchSize() int32 {
	if x != nil {
		return x.BatchSize
	}
	return 0
}

type ProcessRetriesResponse struct {
	state             protoimpl.MessageState `protogen:"open.v1"`
	ProcessedCount    int32                  `protobuf:"varint,1,opt,name=processed_count,json=processedCount,proto3" json:"processed_count,omitempty"`
	ProcessedRetryIds []string               `protobuf:"bytes,2,rep,name=processed_retry_ids,json=processedRetryIds,proto3" json:"processed_retry_ids,omitempty"`
	unknownFields     protoimpl.UnknownFields
	sizeCache         protoimpl.SizeCache
}

func (x *ProcessRetriesResponse) Reset() {
	*x = ProcessRetriesResponse{}
	mi := &file_proto_retry_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ProcessRetriesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProcessRetriesResponse) ProtoMessage() {}

func (x *ProcessRetriesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProcessRetriesResponse.ProtoReflect.Descriptor instead.
func (*ProcessRetriesResponse) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{9}
}

func (x *ProcessRetriesResponse) GetProcessedCount() int32 {
	if x != nil {
		return x.ProcessedCount
	}
	return 0
}

func (x *ProcessRetriesResponse) GetProcessedRetryIds() []string {
	if x != nil {
		return x.ProcessedRetryIds
	}
	return nil
}

// Data Models
type RetryStatus struct {
	state           protoimpl.MessageState `protogen:"open.v1"`
	RetryId         string                 `protobuf:"bytes,1,opt,name=retry_id,json=retryId,proto3" json:"retry_id,omitempty"`
	EventId         string                 `protobuf:"bytes,2,opt,name=event_id,json=eventId,proto3" json:"event_id,omitempty"`
	WebhookId       string                 `protobuf:"bytes,3,opt,name=webhook_id,json=webhookId,proto3" json:"webhook_id,omitempty"`
	CustomerId      string                 `protobuf:"bytes,4,opt,name=customer_id,json=customerId,proto3" json:"customer_id,omitempty"`
	Payload         []byte                 `protobuf:"bytes,5,opt,name=payload,proto3" json:"payload,omitempty"`
	AttemptNumber   int32                  `protobuf:"varint,6,opt,name=attempt_number,json=attemptNumber,proto3" json:"attempt_number,omitempty"`
	MaxAttempts     int32                  `protobuf:"varint,7,opt,name=max_attempts,json=maxAttempts,proto3" json:"max_attempts,omitempty"`
	FailureReason   string                 `protobuf:"bytes,8,opt,name=failure_reason,json=failureReason,proto3" json:"failure_reason,omitempty"`
	State           RetryState             `protobuf:"varint,9,opt,name=state,proto3,enum=retry.RetryState" json:"state,omitempty"`
	CreatedAt       *timestamppb.Timestamp `protobuf:"bytes,10,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
	NextRetryAt     *timestamppb.Timestamp `protobuf:"bytes,11,opt,name=next_retry_at,json=nextRetryAt,proto3" json:"next_retry_at,omitempty"`
	LastAttemptedAt *timestamppb.Timestamp `protobuf:"bytes,12,opt,name=last_attempted_at,json=lastAttemptedAt,proto3" json:"last_attempted_at,omitempty"`
	Headers         map[string]string      `protobuf:"bytes,13,rep,name=headers,proto3" json:"headers,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
	unknownFields   protoimpl.UnknownFields
	sizeCache       protoimpl.SizeCache
}

func (x *RetryStatus) Reset() {
	*x = RetryStatus{}
	mi := &file_proto_retry_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RetryStatus) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RetryStatus) ProtoMessage() {}

func (x *RetryStatus) ProtoReflect() protoreflect.Message {
	mi := &file_proto_retry_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RetryStatus.ProtoReflect.Descriptor instead.
func (*RetryStatus) Descriptor() ([]byte, []int) {
	return file_proto_retry_proto_rawDescGZIP(), []int{10}
}

func (x *RetryStatus) GetRetryId() string {
	if x != nil {
		return x.RetryId
	}
	return ""
}

func (x *RetryStatus) GetEventId() string {
	if x != nil {
		return x.EventId
	}
	return ""
}

func (x *RetryStatus) GetWebhookId() string {
	if x != nil {
		return x.WebhookId
	}
	return ""
}

func (x *RetryStatus) GetCustomerId() string {
	if x != nil {
		return x.CustomerId
	}
	return ""
}

func (x *RetryStatus) GetPayload() []byte {
	if x != nil {
		return x.Payload
	}
	return nil
}

func (x *RetryStatus) GetAttemptNumber() int32 {
	if x != nil {
		return x.AttemptNumber
	}
	return 0
}

func (x *RetryStatus) GetMaxAttempts() int32 {
	if x != nil {
		return x.MaxAttempts
	}
	return 0
}

func (x *RetryStatus) GetFailureReason() string {
	if x != nil {
		return x.FailureReason
	}
	return ""
}

func (x *RetryStatus) GetState() RetryState {
	if x != nil {
		return x.State
	}
	return RetryState_RETRY_STATE_UNSPECIFIED
}

func (x *RetryStatus) GetCreatedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.CreatedAt
	}
	return nil
}

func (x *RetryStatus) GetNextRetryAt() *timestamppb.Timestamp {
	if x != nil {
		return x.NextRetryAt
	}
	return nil
}

func (x *RetryStatus) GetLastAttemptedAt() *timestamppb.Timestamp {
	if x != nil {
		return x.LastAttemptedAt
	}
	return nil
}

func (x *RetryStatus) GetHeaders() map[string]string {
	if x != nil {
		return x.Headers
	}
	return nil
}

var File_proto_retry_proto protoreflect.FileDescriptor

const file_proto_retry_proto_rawDesc = "" +
	"\n" +
	"\x11proto/retry.proto\x12\x05retry\x1a\x1fgoogle/protobuf/timestamp.proto\"\x99\x03\n" +
	"\x14ScheduleRetryRequest\x12\x19\n" +
	"\bevent_id\x18\x01 \x01(\tR\aeventId\x12\x1d\n" +
	"\n" +
	"webhook_id\x18\x02 \x01(\tR\twebhookId\x12\x1f\n" +
	"\vcustomer_id\x18\x03 \x01(\tR\n" +
	"customerId\x12\x18\n" +
	"\apayload\x18\x04 \x01(\fR\apayload\x12%\n" +
	"\x0eattempt_number\x18\x05 \x01(\x05R\rattemptNumber\x12%\n" +
	"\x0efailure_reason\x18\x06 \x01(\tR\rfailureReason\x12>\n" +
	"\rnext_retry_at\x18\a \x01(\v2\x1a.google.protobuf.TimestampR\vnextRetryAt\x12B\n" +
	"\aheaders\x18\b \x03(\v2(.retry.ScheduleRetryRequest.HeadersEntryR\aheaders\x1a:\n" +
	"\fHeadersEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01\"\x8b\x01\n" +
	"\x15ScheduleRetryResponse\x12\x19\n" +
	"\bretry_id\x18\x01 \x01(\tR\aretryId\x12=\n" +
	"\fscheduled_at\x18\x02 \x01(\v2\x1a.google.protobuf.TimestampR\vscheduledAt\x12\x18\n" +
	"\asuccess\x18\x03 \x01(\bR\asuccess\"M\n" +
	"\x15GetRetryStatusRequest\x12\x19\n" +
	"\bretry_id\x18\x01 \x01(\tR\aretryId\x12\x19\n" +
	"\bevent_id\x18\x02 \x01(\tR\aeventId\"D\n" +
	"\x16GetRetryStatusResponse\x12*\n" +
	"\x06status\x18\x01 \x01(\v2\x12.retry.RetryStatusR\x06status\"b\n" +
	"\x12CancelRetryRequest\x12\x19\n" +
	"\bretry_id\x18\x01 \x01(\tR\aretryId\x12\x19\n" +
	"\bevent_id\x18\x02 \x01(\tR\aeventId\x12\x16\n" +
	"\x06reason\x18\x03 \x01(\tR\x06reason\"/\n" +
	"\x13CancelRetryResponse\x12\x18\n" +
	"\asuccess\x18\x01 \x01(\bR\asuccess\"\xab\x01\n" +
	"\x12ListRetriesRequest\x12\x1d\n" +
	"\n" +
	"webhook_id\x18\x01 \x01(\tR\twebhookId\x12\x1f\n" +
	"\vcustomer_id\x18\x02 \x01(\tR\n" +
	"customerId\x12\x14\n" +
	"\x05limit\x18\x03 \x01(\x05R\x05limit\x12\x16\n" +
	"\x06offset\x18\x04 \x01(\x05R\x06offset\x12'\n" +
	"\x05state\x18\x05 \x01(\x0e2\x11.retry.RetryStateR\x05state\"d\n" +
	"\x13ListRetriesResponse\x12,\n" +
	"\aretries\x18\x01 \x03(\v2\x12.retry.RetryStatusR\aretries\x12\x1f\n" +
	"\vtotal_count\x18\x02 \x01(\x05R\n" +
	"totalCount\"6\n" +
	"\x15ProcessRetriesRequest\x12\x1d\n" +
	"\n" +
	"batch_size\x18\x01 \x01(\x05R\tbatchSize\"q\n" +
	"\x16ProcessRetriesResponse\x12'\n" +
	"\x0fprocessed_count\x18\x01 \x01(\x05R\x0eprocessedCount\x12.\n" +
	"\x13processed_retry_ids\x18\x02 \x03(\tR\x11processedRetryIds\"\xf1\x04\n" +
	"\vRetryStatus\x12\x19\n" +
	"\bretry_id\x18\x01 \x01(\tR\aretryId\x12\x19\n" +
	"\bevent_id\x18\x02 \x01(\tR\aeventId\x12\x1d\n" +
	"\n" +
	"webhook_id\x18\x03 \x01(\tR\twebhookId\x12\x1f\n" +
	"\vcustomer_id\x18\x04 \x01(\tR\n" +
	"customerId\x12\x18\n" +
	"\apayload\x18\x05 \x01(\fR\apayload\x12%\n" +
	"\x0eattempt_number\x18\x06 \x01(\x05R\rattemptNumber\x12!\n" +
	"\fmax_attempts\x18\a \x01(\x05R\vmaxAttempts\x12%\n" +
	"\x0efailure_reason\x18\b \x01(\tR\rfailureReason\x12'\n" +
	"\x05state\x18\t \x01(\x0e2\x11.retry.RetryStateR\x05state\x129\n" +
	"\n" +
	"created_at\x18\n" +
	" \x01(\v2\x1a.google.protobuf.TimestampR\tcreatedAt\x12>\n" +
	"\rnext_retry_at\x18\v \x01(\v2\x1a.google.protobuf.TimestampR\vnextRetryAt\x12F\n" +
	"\x11last_attempted_at\x18\f \x01(\v2\x1a.google.protobuf.TimestampR\x0flastAttemptedAt\x129\n" +
	"\aheaders\x18\r \x03(\v2\x1f.retry.RetryStatus.HeadersEntryR\aheaders\x1a:\n" +
	"\fHeadersEntry\x12\x10\n" +
	"\x03key\x18\x01 \x01(\tR\x03key\x12\x14\n" +
	"\x05value\x18\x02 \x01(\tR\x05value:\x028\x01*\xc8\x01\n" +
	"\n" +
	"RetryState\x12\x1b\n" +
	"\x17RETRY_STATE_UNSPECIFIED\x10\x00\x12\x19\n" +
	"\x15RETRY_STATE_SCHEDULED\x10\x01\x12\x1b\n" +
	"\x17RETRY_STATE_IN_PROGRESS\x10\x02\x12\x19\n" +
	"\x15RETRY_STATE_COMPLETED\x10\x03\x12\x16\n" +
	"\x12RETRY_STATE_FAILED\x10\x04\x12\x19\n" +
	"\x15RETRY_STATE_CANCELLED\x10\x05\x12\x17\n" +
	"\x13RETRY_STATE_EXPIRED\x10\x062\x84\x03\n" +
	"\fRetryManager\x12J\n" +
	"\rScheduleRetry\x12\x1b.retry.ScheduleRetryRequest\x1a\x1c.retry.ScheduleRetryResponse\x12M\n" +
	"\x0eGetRetryStatus\x12\x1c.retry.GetRetryStatusRequest\x1a\x1d.retry.GetRetryStatusResponse\x12D\n" +
	"\vCancelRetry\x12\x19.retry.CancelRetryRequest\x1a\x1a.retry.CancelRetryResponse\x12D\n" +
	"\vListRetries\x12\x19.retry.ListRetriesRequest\x1a\x1a.retry.ListRetriesResponse\x12M\n" +
	"\x0eProcessRetries\x12\x1c.retry.ProcessRetriesRequest\x1a\x1d.retry.ProcessRetriesResponseB.Z,github.com/alexnthnz/webhook/proto/generatedb\x06proto3"

var (
	file_proto_retry_proto_rawDescOnce sync.Once
	file_proto_retry_proto_rawDescData []byte
)

func file_proto_retry_proto_rawDescGZIP() []byte {
	file_proto_retry_proto_rawDescOnce.Do(func() {
		file_proto_retry_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_proto_retry_proto_rawDesc), len(file_proto_retry_proto_rawDesc)))
	})
	return file_proto_retry_proto_rawDescData
}

var file_proto_retry_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto_retry_proto_msgTypes = make([]protoimpl.MessageInfo, 13)
var file_proto_retry_proto_goTypes = []any{
	(RetryState)(0),                // 0: retry.RetryState
	(*ScheduleRetryRequest)(nil),   // 1: retry.ScheduleRetryRequest
	(*ScheduleRetryResponse)(nil),  // 2: retry.ScheduleRetryResponse
	(*GetRetryStatusRequest)(nil),  // 3: retry.GetRetryStatusRequest
	(*GetRetryStatusResponse)(nil), // 4: retry.GetRetryStatusResponse
	(*CancelRetryRequest)(nil),     // 5: retry.CancelRetryRequest
	(*CancelRetryResponse)(nil),    // 6: retry.CancelRetryResponse
	(*ListRetriesRequest)(nil),     // 7: retry.ListRetriesRequest
	(*ListRetriesResponse)(nil),    // 8: retry.ListRetriesResponse
	(*ProcessRetriesRequest)(nil),  // 9: retry.ProcessRetriesRequest
	(*ProcessRetriesResponse)(nil), // 10: retry.ProcessRetriesResponse
	(*RetryStatus)(nil),            // 11: retry.RetryStatus
	nil,                            // 12: retry.ScheduleRetryRequest.HeadersEntry
	nil,                            // 13: retry.RetryStatus.HeadersEntry
	(*timestamppb.Timestamp)(nil),  // 14: google.protobuf.Timestamp
}
var file_proto_retry_proto_depIdxs = []int32{
	14, // 0: retry.ScheduleRetryRequest.next_retry_at:type_name -> google.protobuf.Timestamp
	12, // 1: retry.ScheduleRetryRequest.headers:type_name -> retry.ScheduleRetryRequest.HeadersEntry
	14, // 2: retry.ScheduleRetryResponse.scheduled_at:type_name -> google.protobuf.Timestamp
	11, // 3: retry.GetRetryStatusResponse.status:type_name -> retry.RetryStatus
	0,  // 4: retry.ListRetriesRequest.state:type_name -> retry.RetryState
	11, // 5: retry.ListRetriesResponse.retries:type_name -> retry.RetryStatus
	0,  // 6: retry.RetryStatus.state:type_name -> retry.RetryState
	14, // 7: retry.RetryStatus.created_at:type_name -> google.protobuf.Timestamp
	14, // 8: retry.RetryStatus.next_retry_at:type_name -> google.protobuf.Timestamp
	14, // 9: retry.RetryStatus.last_attempted_at:type_name -> google.protobuf.Timestamp
	13, // 10: retry.RetryStatus.headers:type_name -> retry.RetryStatus.HeadersEntry
	1,  // 11: retry.RetryManager.ScheduleRetry:input_type -> retry.ScheduleRetryRequest
	3,  // 12: retry.RetryManager.GetRetryStatus:input_type -> retry.GetRetryStatusRequest
	5,  // 13: retry.RetryManager.CancelRetry:input_type -> retry.CancelRetryRequest
	7,  // 14: retry.RetryManager.ListRetries:input_type -> retry.ListRetriesRequest
	9,  // 15: retry.RetryManager.ProcessRetries:input_type -> retry.ProcessRetriesRequest
	2,  // 16: retry.RetryManager.ScheduleRetry:output_type -> retry.ScheduleRetryResponse
	4,  // 17: retry.RetryManager.GetRetryStatus:output_type -> retry.GetRetryStatusResponse
	6,  // 18: retry.RetryManager.CancelRetry:output_type -> retry.CancelRetryResponse
	8,  // 19: retry.RetryManager.ListRetries:output_type -> retry.ListRetriesResponse
	10, // 20: retry.RetryManager.ProcessRetries:output_type -> retry.ProcessRetriesResponse
	16, // [16:21] is the sub-list for method output_type
	11, // [11:16] is the sub-list for method input_type
	11, // [11:11] is the sub-list for extension type_name
	11, // [11:11] is the sub-list for extension extendee
	0,  // [0:11] is the sub-list for field type_name
}

func init() { file_proto_retry_proto_init() }
func file_proto_retry_proto_init() {
	if File_proto_retry_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_proto_retry_proto_rawDesc), len(file_proto_retry_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   13,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_retry_proto_goTypes,
		DependencyIndexes: file_proto_retry_proto_depIdxs,
		EnumInfos:         file_proto_retry_proto_enumTypes,
		MessageInfos:      file_proto_retry_proto_msgTypes,
	}.Build()
	File_proto_retry_proto = out.File
	file_proto_retry_proto_goTypes = nil
	file_proto_retry_proto_depIdxs = nil
}
