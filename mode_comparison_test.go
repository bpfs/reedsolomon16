package reedsolomon

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

// TestModeComparison 测试内存模式和流式模式的一致性比较
// 按照以下流程测试:
// 1. 使用流式进行分割和生成校验码
// 2. 使用内存模式合并数据片段，验证哈希
// 3. 使用流式模式合并数据片段，验证哈希
// 4. 模拟丢失数据分片，分别使用内存模式和流式模式恢复
// 5. 比较两种模式恢复的结果
func TestModeComparison(t *testing.T) {
	// 使用几种不同的数据大小进行测试
	dataSizes := []int{
		63,    // 比64小1字节
		64,    // 刚好64字节
		65,    // 比64大1字节
		127,   // 比128小1字节
		128,   // 刚好128字节
		32768, // 32KB
	}

	// 固定分片配置
	dataShards := 4
	parityShards := 2

	for _, dataSize := range dataSizes {
		t.Run(fmt.Sprintf("Size_%d", dataSize), func(t *testing.T) {
			testModeComparisonWithSize(t, dataShards, parityShards, dataSize)
		})
	}
}

// testModeComparisonWithSize 使用指定大小进行内存模式和流式模式的比较测试
func testModeComparisonWithSize(t *testing.T, dataShards, parityShards, dataSize int) {
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
	origDataHash := md5Hash(data)
	t.Logf("原始数据大小: %d 字节, 哈希: %s", len(data), origDataHash)

	// 步骤1: 使用流式分割数据并生成校验码
	// ================================================

	t.Log("步骤1: 使用流式分割数据并生成校验码")

	// 创建数据分片缓冲区
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	// 流式拆分数据
	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		t.Fatal("流式拆分失败:", err)
	}

	// 打印数据分片信息
	for i, buf := range dataBuffers {
		t.Logf("数据分片 %d 大小: %d 字节, 哈希: %s", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 创建奇偶校验分片缓冲区
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建数据分片Reader用于编码
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式编码生成奇偶校验分片
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 打印奇偶校验分片信息
	for i, buf := range parityBuffers {
		t.Logf("奇偶校验分片 %d 大小: %d 字节, 哈希: %s", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 验证所有分片
	allReaders := make([]io.Reader, dataShards+parityShards)
	for i := 0; i < dataShards; i++ {
		allReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}
	for i := 0; i < parityShards; i++ {
		allReaders[i+dataShards] = bytes.NewReader(parityBuffers[i].Bytes())
	}

	ok, err := r.StreamVerify(allReaders)
	if err != nil {
		t.Fatal("流式验证失败:", err)
	}
	if !ok {
		t.Fatal("流式验证结果: 分片数据不一致")
	}
	t.Log("流式验证通过: 分片数据一致")

	// 步骤2: 使用内存模式合并数据片段
	// ================================================

	t.Log("步骤2: 使用内存模式合并数据片段")

	// 将数据分片转换为内存模式使用的格式
	memDataShards := make([][]byte, dataShards)
	for i, buf := range dataBuffers {
		memDataShards[i] = buf.Bytes()
	}

	// 内存合并
	var memResult bytes.Buffer
	err = r.Join(&memResult, memDataShards, dataSize)
	if err != nil {
		t.Fatal("内存合并失败:", err)
	}

	// 验证内存合并结果
	memResultData := memResult.Bytes()
	memResultHash := md5Hash(memResultData)
	t.Logf("内存合并结果大小: %d 字节, 哈希: %s", len(memResultData), memResultHash)

	if memResultHash != origDataHash {
		t.Fatal("内存合并结果与原始数据不匹配")
	}
	t.Log("内存合并验证通过: 结果与原始数据一致")

	// 步骤3: 使用流式模式合并数据片段
	// ================================================

	t.Log("步骤3: 使用流式模式合并数据片段")

	// 创建数据分片的Reader用于流式合并
	streamDataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		streamDataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式合并
	var streamResult bytes.Buffer
	err = r.StreamJoin(&streamResult, streamDataReaders, int64(dataSize))
	if err != nil {
		t.Fatal("流式合并失败:", err)
	}

	// 验证流式合并结果
	streamResultData := streamResult.Bytes()
	streamResultHash := md5Hash(streamResultData)
	t.Logf("流式合并结果大小: %d 字节, 哈希: %s", len(streamResultData), streamResultHash)

	if streamResultHash != origDataHash {
		t.Fatal("流式合并结果与原始数据不匹配")
	}
	t.Log("流式合并验证通过: 结果与原始数据一致")

	// 步骤4: 模拟丢失第一个数据分片
	// ================================================

	t.Log("步骤4: 模拟丢失第一个数据分片")

	// 保存原始第一个数据分片
	originalShard0 := append([]byte{}, dataBuffers[0].Bytes()...)
	originalShard0Hash := md5Hash(originalShard0)
	t.Logf("原始数据分片0大小: %d 字节, 哈希: %s", len(originalShard0), originalShard0Hash)

	// 步骤5: 使用内存模式恢复丢失的数据分片
	// ================================================

	t.Log("步骤5: 使用内存模式恢复丢失的数据分片")

	// 准备用于内存重建的分片数组
	memShards := make([][]byte, dataShards+parityShards)
	memShards[0] = nil // 标记第一个数据分片为丢失

	// 复制其他数据分片
	for i := 1; i < dataShards; i++ {
		memShards[i] = append([]byte{}, dataBuffers[i].Bytes()...)
	}

	// 复制奇偶校验分片
	for i := 0; i < parityShards; i++ {
		memShards[i+dataShards] = append([]byte{}, parityBuffers[i].Bytes()...)
	}

	// 使用内存模式重建丢失的数据分片
	err = r.Reconstruct(memShards)
	if err != nil {
		t.Fatal("内存重建失败:", err)
	}

	// 验证内存重建结果
	memReconstructedShard0Hash := md5Hash(memShards[0])
	t.Logf("内存重建数据分片0大小: %d 字节, 哈希: %s", len(memShards[0]), memReconstructedShard0Hash)

	if memReconstructedShard0Hash != originalShard0Hash {
		t.Logf("警告: 内存重建的数据分片0与原始分片哈希不一致")
		// 比较前几个字节
		for i := 0; i < 20 && i < len(memShards[0]) && i < len(originalShard0); i++ {
			t.Logf("位置 %d: 内存重建=%d, 原始=%d", i, memShards[0][i], originalShard0[i])
		}
	} else {
		t.Log("内存重建的数据分片0与原始分片一致")
	}

	// 使用内存重建的分片再次合并
	var memReconResult bytes.Buffer
	err = r.Join(&memReconResult, memShards[:dataShards], dataSize)
	if err != nil {
		t.Fatal("内存重建后合并失败:", err)
	}

	// 验证内存重建后合并结果
	memReconResultData := memReconResult.Bytes()
	memReconResultHash := md5Hash(memReconResultData)
	t.Logf("内存重建后合并结果大小: %d 字节, 哈希: %s", len(memReconResultData), memReconResultHash)

	if memReconResultHash != origDataHash {
		t.Fatal("内存重建后合并结果与原始数据不匹配")
	}
	t.Log("内存重建后合并验证通过: 结果与原始数据一致")

	// 步骤6: 使用流式模式恢复丢失的数据分片
	// ================================================

	t.Log("步骤6: 使用流式模式恢复丢失的数据分片")

	// 准备用于流式重建的输入
	streamInputs := make([]io.Reader, dataShards+parityShards)
	// 第一个输入设为nil，表示丢失
	for i := 1; i < dataShards; i++ {
		streamInputs[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}
	for i := 0; i < parityShards; i++ {
		streamInputs[i+dataShards] = bytes.NewReader(parityBuffers[i].Bytes())
	}

	// 准备输出
	var reconstructedBuffer bytes.Buffer
	streamOutputs := make([]io.Writer, dataShards+parityShards)
	streamOutputs[0] = &reconstructedBuffer // 只重建第一个分片

	// 流式重建
	err = r.StreamReconstruct(streamInputs, streamOutputs)
	if err != nil {
		t.Fatal("流式重建失败:", err)
	}

	// 验证流式重建结果
	streamReconstructedData := reconstructedBuffer.Bytes()
	streamReconstructedHash := md5Hash(streamReconstructedData)
	t.Logf("流式重建数据分片0大小: %d 字节, 哈希: %s", len(streamReconstructedData), streamReconstructedHash)

	if streamReconstructedHash != originalShard0Hash {
		t.Logf("警告: 流式重建的数据分片0与原始分片哈希不一致")
		// 比较前几个字节
		for i := 0; i < 20 && i < len(streamReconstructedData) && i < len(originalShard0); i++ {
			t.Logf("位置 %d: 流式重建=%d, 原始=%d", i, streamReconstructedData[i], originalShard0[i])
		}

		// 还要比较与内存重建的结果
		t.Log("比较内存重建与流式重建结果:")
		for i := 0; i < 20 && i < len(memShards[0]) && i < len(streamReconstructedData); i++ {
			t.Logf("位置 %d: 内存重建=%d, 流式重建=%d",
				i, memShards[0][i], streamReconstructedData[i])
		}
	} else {
		t.Log("流式重建的数据分片0与原始分片一致")
	}

	// 使用流式重建的分片再次流式合并
	streamReconReaders := make([]io.Reader, dataShards)
	streamReconReaders[0] = bytes.NewReader(streamReconstructedData)
	for i := 1; i < dataShards; i++ {
		streamReconReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	var streamReconResult bytes.Buffer
	err = r.StreamJoin(&streamReconResult, streamReconReaders, int64(dataSize))
	if err != nil {
		t.Fatal("流式重建后合并失败:", err)
	}

	// 验证流式重建后合并结果
	streamReconResultData := streamReconResult.Bytes()
	streamReconResultHash := md5Hash(streamReconResultData)
	t.Logf("流式重建后合并结果大小: %d 字节, 哈希: %s", len(streamReconResultData), streamReconResultHash)

	if streamReconResultHash != origDataHash {
		t.Fatal("流式重建后合并结果与原始数据不匹配")
	}
	t.Log("流式重建后合并验证通过: 结果与原始数据一致")

	// 总结
	t.Log("测试完成:")
	t.Log("1. 原始数据哈希:", origDataHash)
	t.Log("2. 内存合并结果哈希:", memResultHash)
	t.Log("3. 流式合并结果哈希:", streamResultHash)
	t.Log("4. 原始分片0哈希:", originalShard0Hash)
	t.Log("5. 内存重建分片0哈希:", memReconstructedShard0Hash)
	t.Log("6. 流式重建分片0哈希:", streamReconstructedHash)
	t.Log("7. 内存重建后合并哈希:", memReconResultHash)
	t.Log("8. 流式重建后合并哈希:", streamReconResultHash)
}
