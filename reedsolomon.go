/**
 * Reed-Solomon 编码库 - 统一入口点
 *
 * Copyright 2024
 */

package reedsolomon

import (
	"errors"
	"io"
)

// 错误定义
var (
	ErrInvShardNum         = errors.New("无效的分片数量")
	ErrMaxShardNum         = errors.New("分片数量超过最大支持数")
	ErrTooFewShards        = errors.New("可用分片数量不足，无法重建数据")
	ErrShardNoData         = errors.New("分片中没有数据")
	ErrShardSize           = errors.New("分片大小不一致")
	ErrEmptyShards         = errors.New("空分片数组")
	ErrInvalidShards       = errors.New("无效的分片数据")
	ErrInvalidInput        = errors.New("无效的输入数据")
	ErrInvalidOutput       = errors.New("无效的输出缓冲区")
	ErrReconstructRequired = errors.New("需要先进行数据重建")
	ErrInvalidShardSize    = errors.New("分片大小不满足要求，通常是N的倍数")
	ErrShortData           = errors.New("数据不足，无法填充请求的分片数量")
	ErrNotSupported        = errors.New("operation not supported")
	// 流式操作相关错误
	ErrReconstructMismatch = errors.New("一个分片不能同时是输入和输出")
	ErrNilWriter           = errors.New("目标写入器不能为nil")
	ErrSize                = errors.New("无效的大小参数")
)

// ReedSolomon 接口定义了Reed-Solomon编解码器的通用操作
// 支持内存操作和流式操作
type ReedSolomon interface {
	// 获取配置信息
	DataShards() int   // 返回数据分片数量
	ParityShards() int // 返回奇偶校验分片数量
	TotalShards() int  // 返回总分片数量（数据分片+奇偶校验分片）

	// 内存操作
	Encode(shards [][]byte) error                           // 对数据分片编码，生成奇偶校验分片
	Verify(shards [][]byte) (bool, error)                   // 验证分片数据的一致性
	Reconstruct(shards [][]byte) error                      // 重建丢失的分片（数据和奇偶校验）
	ReconstructData(shards [][]byte) error                  // 只重建丢失的数据分片
	Split(data []byte) ([][]byte, error)                    // 将数据拆分成多个分片
	Join(dst io.Writer, shards [][]byte, outSize int) error // 将分片合并成单个数据块

	// 流式操作
	StreamEncode(inputs []io.Reader, outputs []io.Writer) error          // 流式编码
	StreamVerify(shards []io.Reader) (bool, error)                       // 流式验证
	StreamReconstruct(inputs []io.Reader, outputs []io.Writer) error     // 流式重建
	StreamReconstructData(inputs []io.Reader, outputs []io.Writer) error // 流式重建数据分片
	StreamSplit(data io.Reader, dst []io.Writer, size int64) error       // 流式拆分
	StreamJoin(dst io.Writer, shards []io.Reader, outSize int64) error   // 流式合并

	// 内存管理
	AllocAligned(shards, each int) [][]byte // 分配对齐的内存
	ShardSizeMultiple() int                 // 返回分片大小需要满足的倍数

	// 并发控制
	WithConcurrency(n int) ReedSolomon // 设置并发级别
}

// New 创建一个新的Reed-Solomon编解码器
// 如果总分片数 <= 256，将使用GF(2^8)实现，否则使用GF(2^16)实现
func New(dataShards, parityShards int) (ReedSolomon, error) {
	if dataShards <= 0 || parityShards <= 0 {
		return nil, ErrInvShardNum
	}

	totalShards := dataShards + parityShards

	// 根据分片数量选择合适的实现
	if totalShards <= 256 {
		return New8(dataShards, parityShards)
	}
	return New16(dataShards, parityShards)
}

// New8 创建一个基于GF(2^8)的Reed-Solomon编解码器，最多支持256个分片
func New8(dataShards, parityShards int) (ReedSolomon, error) {
	// 调用内部实现函数
	return newReedSolomon8(dataShards, parityShards)
}

// New16 创建一个基于GF(2^16)的Reed-Solomon编解码器，最多支持65535个分片
func New16(dataShards, parityShards int) (ReedSolomon, error) {
	// 调用内部实现函数
	return newReedSolomon16(dataShards, parityShards)
}

// 包装 leopardFF8 的结构体，实现完整的 ReedSolomon 接口
type rsFF8 struct {
	*leopardFF8
}

// 包装 leopardFF16 的结构体，实现完整的 ReedSolomon 接口
type rsFF16 struct {
	*leopardFF16
}

// AllocAligned 实现 ReedSolomon 接口中的 AllocAligned 方法
func (r *rsFF8) AllocAligned(shards, each int) [][]byte {
	// 如果 shards 参数与内部 totalShards 不同，则使用传入的 shards 参数
	if shards != r.totalShards {
		return AllocAligned(shards, each)
	}
	return r.leopardFF8.AllocAligned(each)
}

// AllocAligned 实现 ReedSolomon 接口中的 AllocAligned 方法
func (r *rsFF16) AllocAligned(shards, each int) [][]byte {
	// 如果 shards 参数与内部 totalShards 不同，则使用传入的 shards 参数
	if shards != r.totalShards {
		return AllocAligned(shards, each)
	}
	return r.leopardFF16.AllocAligned(each)
}

// 以下方法是流式接口的实现，使用rsStream8
func (r *rsFF8) StreamEncode(inputs []io.Reader, outputs []io.Writer) error {
	if len(inputs) != r.dataShards {
		return ErrTooFewShards
	}
	if len(outputs) != r.parityShards {
		return ErrTooFewShards
	}

	enc, err := newStreamEncoderFF8(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	return enc.encode(inputs, outputs)
}

// StreamVerify验证经过编码的数据分片和奇偶校验分片正确性，通过Readers读取数据
func (r *rsFF8) StreamVerify(shards []io.Reader) (bool, error) {
	if len(shards) != r.totalShards {
		return false, ErrTooFewShards
	}

	// 创建流式编码器
	enc, err := newStreamEncoderFF8(r.dataShards, r.parityShards)
	if err != nil {
		return false, err
	}

	// 执行验证
	return enc.verify(shards)
}

func (r *rsFF8) StreamReconstruct(inputs []io.Reader, outputs []io.Writer) error {
	if len(inputs) != r.totalShards || len(outputs) != r.totalShards {
		return ErrTooFewShards
	}

	// 确保不会同时尝试从同一个分片读取和写入
	for i := range inputs {
		if inputs[i] != nil && outputs[i] != nil {
			return ErrReconstructMismatch
		}
	}

	// 创建流式编码器
	enc, err := newStreamEncoderFF8(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	// 确定是否只需要重建数据分片
	onlyData := true
	for i := r.dataShards; i < r.totalShards; i++ {
		if outputs[i] != nil {
			onlyData = false
			break
		}
	}

	// 执行相应的重建
	if onlyData {
		return enc.reconstructData(inputs, outputs)
	} else {
		return enc.reconstruct(inputs, outputs)
	}
}

func (r *rsFF8) StreamReconstructData(inputs []io.Reader, outputs []io.Writer) error {
	// 创建一个新的输出切片，确保只标记数据分片进行重建
	dataOnlyOutputs := make([]io.Writer, r.totalShards)

	// 复制数据分片的输出
	for i := 0; i < r.dataShards; i++ {
		dataOnlyOutputs[i] = outputs[i]
	}

	// 调用完整的重建方法
	return r.StreamReconstruct(inputs, dataOnlyOutputs)
}

func (r *rsFF8) StreamSplit(data io.Reader, dst []io.Writer, size int64) error {
	if len(dst) != r.dataShards {
		return ErrTooFewShards
	}

	enc, err := newStreamEncoderFF8(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	return enc.split(data, dst, size)
}

func (r *rsFF8) StreamJoin(dst io.Writer, shards []io.Reader, outSize int64) error {
	if dst == nil {
		return ErrNilWriter
	}

	enc, err := newStreamEncoderFF8(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	return enc.join(dst, shards, outSize)
}

// 以下方法是流式接口的实现，使用rsStream16
func (r *rsFF16) StreamEncode(inputs []io.Reader, outputs []io.Writer) error {
	if len(inputs) != r.dataShards {
		return ErrTooFewShards
	}

	if len(outputs) != r.parityShards {
		return ErrTooFewShards
	}

	enc, err := newStreamEncoderFF16(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	return enc.encode(inputs, outputs)
}

// StreamVerify验证经过编码的数据分片和奇偶校验分片正确性，通过Readers读取数据
func (r *rsFF16) StreamVerify(shards []io.Reader) (bool, error) {
	if len(shards) != r.totalShards {
		return false, ErrTooFewShards
	}

	// 创建流式编码器
	enc, err := newStreamEncoderFF16(r.dataShards, r.parityShards)
	if err != nil {
		return false, err
	}

	// 执行验证
	return enc.verify(shards)
}

func (r *rsFF16) StreamReconstruct(inputs []io.Reader, outputs []io.Writer) error {
	if len(inputs) != r.totalShards || len(outputs) != r.totalShards {
		return ErrTooFewShards
	}

	// 确保不会同时尝试从同一个分片读取和写入
	for i := range inputs {
		if inputs[i] != nil && outputs[i] != nil {
			return ErrReconstructMismatch
		}
	}

	// 创建流式编码器
	enc, err := newStreamEncoderFF16(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	// 确定是否只需要重建数据分片
	onlyData := true
	for i := r.dataShards; i < r.totalShards; i++ {
		if outputs[i] != nil {
			onlyData = false
			break
		}
	}

	// 执行相应的重建
	if onlyData {
		return enc.reconstructData(inputs, outputs)
	} else {
		return enc.reconstruct(inputs, outputs)
	}
}

func (r *rsFF16) StreamReconstructData(inputs []io.Reader, outputs []io.Writer) error {
	// 创建一个新的输出切片，确保只标记数据分片进行重建
	dataOnlyOutputs := make([]io.Writer, r.totalShards)

	// 复制数据分片的输出
	for i := 0; i < r.dataShards; i++ {
		dataOnlyOutputs[i] = outputs[i]
	}

	// 调用完整的重建方法
	return r.StreamReconstruct(inputs, dataOnlyOutputs)
}

func (r *rsFF16) StreamSplit(data io.Reader, dst []io.Writer, size int64) error {
	if len(dst) != r.dataShards {
		return ErrTooFewShards
	}

	enc, err := newStreamEncoderFF16(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	return enc.split(data, dst, size)
}

func (r *rsFF16) StreamJoin(dst io.Writer, shards []io.Reader, outSize int64) error {
	if dst == nil {
		return ErrNilWriter
	}

	enc, err := newStreamEncoderFF16(r.dataShards, r.parityShards)
	if err != nil {
		return err
	}

	return enc.join(dst, shards, outSize)
}

// newReedSolomon8 创建基于GF(2^8)的Reed-Solomon编解码器的内部实现
func newReedSolomon8(dataShards, parityShards int) (ReedSolomon, error) {
	ff8, err := newFF8(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	return &rsFF8{ff8}, nil
}

// newReedSolomon16 创建基于GF(2^16)的Reed-Solomon编解码器的内部实现
func newReedSolomon16(dataShards, parityShards int) (ReedSolomon, error) {
	ff16, err := newFF16(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	return &rsFF16{ff16}, nil
}

// Extensions is an optional interface.
// All returned instances will support this interface.
type Extensions interface {
	// ShardSizeMultiple will return the size the shard sizes must be a multiple of.
	ShardSizeMultiple() int

	// DataShards will return the number of data shards.
	DataShards() int

	// ParityShards will return the number of parity shards.
	ParityShards() int

	// TotalShards will return the total number of shards.
	TotalShards() int

	// AllocAligned will allocate TotalShards number of slices,
	// aligned to reasonable memory sizes.
	// Provide the size of each shard.
	AllocAligned(each int) [][]byte
}

// 额外接口定义

// StreamEncoder8 是一个基于GF(2^8)的Reed-Solomon流式编码器接口
type StreamEncoder8 interface {
	// Encode 为一组数据分片生成奇偶校验分片
	Encode(inputs []io.Reader, outputs []io.Writer) error

	// Verify 验证奇偶校验分片的正确性
	Verify(shards []io.Reader) (bool, error)

	// Reconstruct 重建丢失的分片
	Reconstruct(inputs []io.Reader, outputs []io.Writer) error

	// Split 将输入流分割成多个分片
	Split(data io.Reader, dst []io.Writer, size int64) error

	// Join 将分片连接起来并将数据段写入dst
	Join(dst io.Writer, shards []io.Reader, outSize int64) error
}

// StreamEncoder16 是一个基于GF(2^16)的Reed-Solomon流式编码器接口
type StreamEncoder16 interface {
	// Encode 为一组数据分片生成奇偶校验分片
	Encode(inputs []io.Reader, outputs []io.Writer) error

	// Verify 验证奇偶校验分片的正确性
	Verify(shards []io.Reader) (bool, error)

	// Reconstruct 重建丢失的分片
	Reconstruct(inputs []io.Reader, outputs []io.Writer) error

	// Split 将输入流分割成多个分片
	Split(data io.Reader, dst []io.Writer, size int64) error

	// Join 将分片连接起来并将数据段写入dst
	Join(dst io.Writer, shards []io.Reader, outSize int64) error
}

// WithConcurrency 实现 ReedSolomon 接口中的 WithConcurrency 方法
func (r *rsFF8) WithConcurrency(n int) ReedSolomon {
	// 目前 leopardFF8 可能没有实现 WithConcurrency
	// 因此只返回自身实例
	return r
}

// WithConcurrency 实现 ReedSolomon 接口中的 WithConcurrency 方法
func (r *rsFF16) WithConcurrency(n int) ReedSolomon {
	// 目前 leopardFF16 可能没有实现 WithConcurrency
	// 因此只返回自身实例
	return r
}
