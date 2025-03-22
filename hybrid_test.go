package reedsolomon

import (
	"bytes"
	"io"
	"testing"
)

// TestHybridReconstruction 测试流式分片+内存重建与流式分片+流式重建的对比
func TestHybridReconstruction(t *testing.T) {
	// 参数设置
	dataShards := 4
	parityShards := 2
	dataSize := 1024

	// 创建编码器
	r, err := New16(dataShards, parityShards)
	if err != nil {
		t.Fatal(err)
	}

	// 创建固定的测试数据
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// 第一步: 流式分片
	// 创建数据分片缓冲区
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	// 拆分数据到分片
	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		t.Fatal("流式拆分失败:", err)
	}

	// 输出分片信息
	t.Log("分片信息:")
	for i, buf := range dataBuffers {
		t.Logf("数据分片 %d 大小: %d", i, buf.Len())
	}

	// 创建奇偶校验分片缓冲区
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建数据分片的Reader用于编码
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 编码生成奇偶校验分片
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 输出奇偶校验分片信息
	for i, buf := range parityBuffers {
		t.Logf("奇偶校验分片 %d 大小: %d", i, buf.Len())
	}

	// 保存所有流式生成的分片数据用于后续测试
	allStreamShards := make([][]byte, dataShards+parityShards)
	for i, buf := range dataBuffers {
		allStreamShards[i] = buf.Bytes()
	}
	for i, buf := range parityBuffers {
		allStreamShards[i+dataShards] = buf.Bytes()
	}

	// 第二步: 使用内存重建模式
	// 创建一个新的切片数组，其中第一个数据分片设为nil
	memoryShards := make([][]byte, dataShards+parityShards)
	for i := range memoryShards {
		if i == 0 {
			memoryShards[i] = nil // 删除第一个数据分片，准备重建
		} else {
			memoryShards[i] = append([]byte{}, allStreamShards[i]...)
		}
	}

	// 输出重建前内存模式的状态
	t.Log("内存重建前状态:")
	for i, shard := range memoryShards {
		if shard == nil {
			t.Logf("分片 %d: nil", i)
		} else {
			t.Logf("分片 %d: 大小=%d", i, len(shard))
		}
	}

	// 使用内存模式的重建函数
	origShard0 := append([]byte{}, allStreamShards[0]...)
	err = r.Reconstruct(memoryShards)
	if err != nil {
		t.Fatal("内存重建失败:", err)
	}

	// 验证内存重建结果
	if !bytes.Equal(memoryShards[0], origShard0) {
		t.Log("内存重建的分片与原始分片不匹配!")
		t.Logf("内存重建分片大小=%d, 原始分片大小=%d", len(memoryShards[0]), len(origShard0))

		// 输出前10个字节进行比较
		for i := 0; i < 10 && i < len(memoryShards[0]) && i < len(origShard0); i++ {
			t.Logf("位置 %d: 重建=%d, 原始=%d", i, memoryShards[0][i], origShard0[i])
		}
		t.Fatal("内存重建的数据分片与原始数据不匹配")
	}

	t.Log("内存重建测试通过!")

	// 第三步: 流式重建测试
	// 创建输入流
	inputReaders := make([]io.Reader, dataShards+parityShards)
	inputReaders[0] = nil // 删除第一个分片
	for i := 1; i < dataShards+parityShards; i++ {
		inputReaders[i] = bytes.NewReader(allStreamShards[i])
	}

	// 创建输出流
	outputBuffers := make([]*bytes.Buffer, dataShards+parityShards)
	outputWriters := make([]io.Writer, dataShards+parityShards)
	outputBuffers[0] = new(bytes.Buffer)
	outputWriters[0] = outputBuffers[0]

	// 输出流式重建前的状态
	t.Log("流式重建前状态:")
	for i, reader := range inputReaders {
		if reader == nil {
			t.Logf("输入 %d: nil", i)
		} else if br, ok := reader.(*bytes.Reader); ok {
			t.Logf("输入 %d: 大小=%d", i, br.Size())
		}
	}

	// 执行流式重建
	err = r.StreamReconstruct(inputReaders, outputWriters)
	if err != nil {
		t.Fatal("流式重建失败:", err)
	}

	// 验证流式重建结果
	streamReconstructed := outputBuffers[0].Bytes()
	if !bytes.Equal(streamReconstructed, origShard0) {
		t.Log("流式重建的分片与原始分片不匹配!")
		t.Logf("流式重建大小=%d, 原始分片大小=%d", len(streamReconstructed), len(origShard0))

		// 找出不匹配的位置
		minLen := len(streamReconstructed)
		if len(origShard0) < minLen {
			minLen = len(origShard0)
		}

		// 输出前20个字节进行比较
		for i := 0; i < 20 && i < minLen; i++ {
			t.Logf("位置 %d: 流式重建=%d, 原始=%d", i, streamReconstructed[i], origShard0[i])
		}

		// 检查内存重建和流式重建的区别
		t.Log("内存重建与流式重建比较:")
		for i := 0; i < 20 && i < len(memoryShards[0]) && i < len(streamReconstructed); i++ {
			t.Logf("位置 %d: 内存重建=%d, 流式重建=%d",
				i, memoryShards[0][i], streamReconstructed[i])
		}

		t.Fatal("流式重建的数据与原始数据不匹配")
	}

	t.Log("流式重建测试通过!")
}
