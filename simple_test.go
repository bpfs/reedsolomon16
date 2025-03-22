package reedsolomon

import (
	"bytes"
	"io"
	"testing"
)

// TestSimpleReconstruction 是一个简化的重建测试，用于排查问题
func TestSimpleReconstruction(t *testing.T) {
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

	// 创建所有分片
	shards := make([][]byte, dataShards+parityShards)
	for i := 0; i < dataShards+parityShards; i++ {
		shards[i] = make([]byte, dataSize/dataShards)
	}

	// 分割数据到数据分片
	for i := 0; i < dataShards; i++ {
		start := i * (dataSize / dataShards)
		end := (i + 1) * (dataSize / dataShards)
		if end > dataSize {
			end = dataSize
		}
		copy(shards[i], data[start:end])
	}

	// 输出分片大小
	for i, shard := range shards {
		t.Logf("分片 %d 大小: %d", i, len(shard))
	}

	// 编码生成奇偶校验分片
	err = r.Encode(shards)
	if err != nil {
		t.Fatal("编码失败:", err)
	}

	// 确认编码正确
	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal("验证失败:", err)
	}
	if !ok {
		t.Fatal("验证结果不一致")
	}

	// 保存第一个数据分片
	origShard0 := append([]byte{}, shards[0]...)

	// 尝试第一种情况：删除一个数据分片
	shards[0] = nil
	err = r.Reconstruct(shards)
	if err != nil {
		t.Fatal("重建失败:", err)
	}

	// 确认重建正确
	if !bytes.Equal(shards[0], origShard0) {
		t.Logf("重建的分片大小: %d, 原始分片大小: %d", len(shards[0]), len(origShard0))
		if len(shards[0]) > 0 && len(origShard0) > 0 {
			for i := 0; i < 10 && i < len(shards[0]) && i < len(origShard0); i++ {
				t.Logf("字节 %d: 重建=%d, 原始=%d", i, shards[0][i], origShard0[i])
			}
		}
		t.Fatal("重建的数据分片与原始数据不匹配")
	}

	t.Log("内存模式下重建测试通过")

	// 测试流式重建
	testStreamSimpleReconstruction(t)
}

// testStreamSimpleReconstruction 测试流式重建
func testStreamSimpleReconstruction(t *testing.T) {
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

	// 创建数据分片缓冲区
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	// 拆分数据
	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(len(data)))
	if err != nil {
		t.Fatal("流式拆分失败:", err)
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

	// 编码
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 输出分片信息
	t.Log("分片信息:")
	for i, buf := range dataBuffers {
		t.Logf("数据分片 %d 大小: %d", i, buf.Len())
	}
	for i, buf := range parityBuffers {
		t.Logf("奇偶校验分片 %d 大小: %d", i, buf.Len())
	}

	// 提取数据和奇偶校验分片
	originalData := make([][]byte, dataShards)
	for i, buf := range dataBuffers {
		originalData[i] = append([]byte{}, buf.Bytes()...)
	}

	originalParity := make([][]byte, parityShards)
	for i, buf := range parityBuffers {
		originalParity[i] = append([]byte{}, buf.Bytes()...)
	}

	// 验证正确性
	allShards := make([]io.Reader, dataShards+parityShards)
	for i := range dataBuffers {
		allShards[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}
	for i := range parityBuffers {
		allShards[i+dataShards] = bytes.NewReader(parityBuffers[i].Bytes())
	}

	ok, err := r.StreamVerify(allShards)
	if err != nil {
		t.Fatal("流式验证失败:", err)
	}
	if !ok {
		t.Fatal("验证结果不一致")
	}

	// 测试重建
	// 删除第一个数据分片
	inputReaders := make([]io.Reader, dataShards+parityShards)
	inputReaders[0] = nil // 模拟丢失
	for i := 1; i < dataShards; i++ {
		inputReaders[i] = bytes.NewReader(originalData[i])
	}
	for i := 0; i < parityShards; i++ {
		inputReaders[i+dataShards] = bytes.NewReader(originalParity[i])
	}

	// 输出
	outputBuffers := make([]*bytes.Buffer, dataShards+parityShards)
	outputWriters := make([]io.Writer, dataShards+parityShards)
	outputBuffers[0] = new(bytes.Buffer)
	outputWriters[0] = outputBuffers[0]

	// 确保输入状态
	t.Log("重建前的状态:")
	for i, reader := range inputReaders {
		if reader == nil {
			t.Logf("输入 %d: nil", i)
		} else if br, ok := reader.(*bytes.Reader); ok {
			t.Logf("输入 %d: 大小=%d", i, br.Size())
		}
	}

	// 执行重建
	err = r.StreamReconstruct(inputReaders, outputWriters)
	if err != nil {
		t.Fatal("流式重建失败:", err)
	}

	// 验证重建结果
	reconstructedData := outputBuffers[0].Bytes()
	if !bytes.Equal(reconstructedData, originalData[0]) {
		t.Logf("重建数据大小=%d, 原始数据大小=%d", len(reconstructedData), len(originalData[0]))

		// 找出不匹配的位置
		minLen := len(reconstructedData)
		if len(originalData[0]) < minLen {
			minLen = len(originalData[0])
		}

		for j := 0; j < minLen && j < 20; j++ {
			t.Logf("位置 %d: 重建=%d, 原始=%d", j, reconstructedData[j], originalData[0][j])
		}

		t.Fatal("流式重建的数据与原始数据不匹配")
	}

	t.Log("流式重建测试通过")
}
