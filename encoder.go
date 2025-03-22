/**
 * Reed-Solomon 编码库 - 编码引擎基础
 *
 * Copyright 2024
 */

package reedsolomon

import (
	"io"
	"sync"
)

// encoderBase 结构体包含了所有编码器实现的通用部分
type encoderBase struct {
	dataShards   int       // 数据分片数量
	parityShards int       // 奇偶校验分片数量
	totalShards  int       // 总分片数量 (dataShards + parityShards)
	workPool     sync.Pool // 工作缓冲区池
}

// DataShards 返回数据分片数量
func (e *encoderBase) DataShards() int {
	return e.dataShards
}

// ParityShards 返回奇偶校验分片数量
func (e *encoderBase) ParityShards() int {
	return e.parityShards
}

// TotalShards 返回总分片数量
func (e *encoderBase) TotalShards() int {
	return e.totalShards
}

// ShardSizeMultiple 返回分片大小需要满足的倍数
// 基础实现返回1，子类可以覆盖
func (e *encoderBase) ShardSizeMultiple() int {
	return 1
}

// checkStreamShards 验证流分片数组的有效性
// 用于流式操作
func checkStreamShards(shards []io.Reader) error {
	if len(shards) == 0 {
		return ErrEmptyShards
	}

	// 确保至少有一个非nil的流
	hasData := false
	for _, shard := range shards {
		if shard != nil {
			hasData = true
			break
		}
	}

	if !hasData {
		return ErrShardNoData
	}

	return nil
}

// Init 初始化编码器基础
// 由子类调用
func (e *encoderBase) init(dataShards, parityShards int) {
	e.dataShards = dataShards
	e.parityShards = parityShards
	e.totalShards = dataShards + parityShards
}

// 以下为常用辅助方法 //

// copyBytes 将一个字节切片复制到另一个字节切片
func copyBytes(dst, src []byte) {
	copy(dst, src)
}

// isZero 检查字节切片是否为全零
func isZero(p []byte) bool {
	for _, v := range p {
		if v != 0 {
			return false
		}
	}
	return true
}

// createShardsBySize 创建指定大小的分片数组
func createShardsBySize(shards int, size int) [][]byte {
	result := make([][]byte, shards)
	for i := range result {
		result[i] = make([]byte, size)
	}
	return result
}

// checkShards will check if shards are the same size or 0, if allowed. An error is returned if this fails.
// An error is also returned if all shards are size 0.
func checkShards(shards [][]byte, nilok bool) error {
	size := shardSize(shards)
	if size == 0 {
		return ErrShardNoData
	}
	for _, shard := range shards {
		if len(shard) != size {
			if len(shard) != 0 || !nilok {
				return ErrShardSize
			}
		}
	}
	return nil
}

// shardSize return the size of a single shard.
// The first non-zero size is returned, or 0 if all shards are size 0.
func shardSize(shards [][]byte) int {
	for _, shard := range shards {
		if len(shard) != 0 {
			return len(shard)
		}
	}
	return 0
}

const (
	codeGenMinSize           = 64
	codeGenMinShards         = 3
	gfniCodeGenMaxGoroutines = 4

	intSize = 32 << (^uint(0) >> 63) // 32 or 64
	maxInt  = 1<<(intSize-1) - 1
)
