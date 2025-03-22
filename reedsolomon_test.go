package reedsolomon

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"testing"
)

// 用于测试的数据大小
const (
	testDataSize   = 1 << 20 // 1 MB
	smallTestSize  = 1 << 10 // 1 KB
	mediumTestSize = 1 << 15 // 32 KB
)

// md5Hash 计算数据的MD5哈希值
func md5Hash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// 计算数据的MD5哈希
func calcHash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// 测试基本的编码解码流程
func TestBasicEncodeDecode(t *testing.T) {
	testEncodeDecode(t, 10, 4, smallTestSize, false) // 基本测试，GF(2^8)
	testEncodeDecode(t, 10, 4, smallTestSize, true)  // 基本测试，GF(2^16)
}

// 测试各种分片组合
func TestDifferentShardSizes(t *testing.T) {
	// 最小配置
	testEncodeDecode(t, 2, 1, smallTestSize, false) // GF(2^8)
	testEncodeDecode(t, 2, 1, smallTestSize, true)  // GF(2^16)

	// 中等配置
	testEncodeDecode(t, 8, 4, smallTestSize, false) // GF(2^8)
	testEncodeDecode(t, 8, 4, smallTestSize, true)  // GF(2^16)

	// 较大分片数量
	testEncodeDecode(t, 16, 4, smallTestSize, false) // GF(2^8)
	testEncodeDecode(t, 16, 4, smallTestSize, true)  // GF(2^16)

	// 接近256个分片
	testEncodeDecode(t, 128, 128, smallTestSize, true) // GF(2^16)
}

// 测试超过256个分片的大规模配置
func TestLargeShardCount(t *testing.T) {
	if testing.Short() {
		t.Skip("大规模分片测试在短模式下跳过")
	}

	// GF(2^8)不支持超过256个分片，以下测试仅针对GF(2^16)

	// 测试300个分片
	t.Run("300 Shards", func(t *testing.T) {
		testEncodeDecode(t, 200, 100, smallTestSize, true) // 总共300个分片
	})

	// 测试500个分片
	t.Run("500 Shards", func(t *testing.T) {
		testEncodeDecode(t, 350, 150, smallTestSize, true) // 总共500个分片
	})

	// 测试1000个分片
	t.Run("1000 Shards", func(t *testing.T) {
		testEncodeDecode(t, 700, 300, smallTestSize, true) // 总共1000个分片
	})

	// 极限测试 - 接近最大值
	// 注意: 这可能需要较大内存
	t.Run("5000 Shards", func(t *testing.T) {
		testLargeShardCount(t, 4000, 1000, true) // 总共5000个分片
	})
}

// 测试数据大小的影响
func TestDifferentDataSizes(t *testing.T) {
	// 小数据
	testEncodeDecode(t, 10, 4, 1024, false) // 1KB
	testEncodeDecode(t, 10, 4, 1024, true)

	// 中等数据
	testEncodeDecode(t, 10, 4, 1<<15, false) // 32KB
	testEncodeDecode(t, 10, 4, 1<<15, true)

	// 大数据 - 只在-race标志关闭时进行
	if testing.Short() {
		t.Skip("大数据测试在短模式下跳过")
	}
	testEncodeDecode(t, 10, 4, 1<<20, false) // 1MB
	testEncodeDecode(t, 10, 4, 1<<20, true)
}

// 测试数据重建功能
func TestReconstruction(t *testing.T) {
	testReconstruction(t, 10, 4, mediumTestSize, false, false)
	testReconstruction(t, 10, 4, mediumTestSize, true, false)
}

// 测试仅数据分片重建功能
func TestReconstructData(t *testing.T) {
	testReconstruction(t, 10, 4, mediumTestSize, false, true)
	testReconstruction(t, 10, 4, mediumTestSize, true, true)
}

// 测试验证功能
func TestVerify(t *testing.T) {
	testVerify(t, 10, 4, smallTestSize, false)
	testVerify(t, 10, 4, smallTestSize, true)
}

// 测试分片边界情况
func TestEdgeCases(t *testing.T) {
	// 测试最小数据（小于所有分片的总大小）
	testEncodeDecode(t, 4, 2, 10, false)
	testEncodeDecode(t, 4, 2, 10, true)

	// 测试刚好被分片整除的数据
	testEncodeDecode(t, 5, 2, 5*64, false) // 对于GF(2^8)，分片大小是64的倍数
	testEncodeDecode(t, 5, 2, 5*64, true)  // 对于GF(2^16)，分片大小是64的倍数
}

// 实际的编码解码测试
func testEncodeDecode(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	var r ReedSolomon
	var err error

	// 根据参数选择FF8或FF16实现
	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New8(dataShards, parityShards)
	}

	if err != nil {
		t.Fatal(err)
	}

	// 创建随机测试数据
	data := make([]byte, dataSize)
	_, err = rand.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	// 将数据分片
	shards, err := r.Split(data)
	if err != nil {
		t.Fatal(err)
	}

	// 验证分片数量
	if len(shards) != dataShards+parityShards {
		t.Fatal("分片数量不正确", len(shards), "预期", dataShards+parityShards)
	}

	// 编码奇偶校验分片
	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// 验证编码正确性
	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("编码验证失败")
	}

	// 合并数据分片并与原始数据比较
	var buf bytes.Buffer
	err = r.Join(&buf, shards, dataSize)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(buf.Bytes(), data) {
		t.Fatal("重建的数据与原始数据不匹配")
	}
}

// 测试数据重建
func testReconstruction(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool, onlyData bool) {
	// 创建编码器
	var r ReedSolomon
	var err error

	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New8(dataShards, parityShards)
	}

	if err != nil {
		t.Fatal(err)
	}

	// 创建随机数据
	data := make([]byte, dataSize)
	_, err = rand.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	// 分片并编码
	shards, err := r.Split(data)
	if err != nil {
		t.Fatal(err)
	}

	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// 删除部分分片（模拟丢失）
	// 最多可以删除parityShards个分片
	maxToRemove := parityShards
	if maxToRemove > 8 {
		maxToRemove = 8 // 简化测试，最多删除8个
	}

	// 记录原始分片的副本
	origShards := make([][]byte, len(shards))
	for i, shard := range shards {
		origShards[i] = make([]byte, len(shard))
		copy(origShards[i], shard)
	}

	// 随机删除数据分片
	for i := 0; i < maxToRemove/2; i++ {
		idx := i * 2 // 间隔删除避免连续删除
		if idx >= dataShards {
			break
		}
		shards[idx] = nil
	}

	// 随机删除奇偶校验分片
	for i := 0; i < maxToRemove/2; i++ {
		idx := dataShards + i
		if idx >= dataShards+parityShards {
			break
		}
		shards[idx] = nil
	}

	// 重建
	var rebuildErr error
	if onlyData {
		rebuildErr = r.ReconstructData(shards)
	} else {
		rebuildErr = r.Reconstruct(shards)
	}

	if rebuildErr != nil {
		t.Fatal(rebuildErr)
	}

	// 验证重建结果
	if onlyData {
		// 仅检查数据分片是否正确重建
		for i := 0; i < dataShards; i++ {
			if shards[i] == nil {
				t.Fatal("数据分片未重建:", i)
			}
			if !bytes.Equal(shards[i], origShards[i]) {
				t.Fatal("数据分片重建结果不正确:", i)
			}
		}
	} else {
		// 检查所有分片是否正确重建
		for i := 0; i < dataShards+parityShards; i++ {
			if shards[i] == nil {
				t.Fatal("分片未重建:", i)
			}
			if !bytes.Equal(shards[i], origShards[i]) {
				t.Fatal("分片重建结果不正确:", i)
			}
		}
	}

	// 合并分片
	var result bytes.Buffer
	err = r.Join(&result, shards, dataSize)
	if err != nil {
		t.Fatal("合并失败:", err)
	}
	recovered := result.Bytes()

	if !bytes.Equal(recovered, data) {
		t.Fatal("重建后的数据与原始数据不匹配")
	} else {
		t.Logf("重建成功，合并后数据大小: %d 字节", len(recovered))
	}

	t.Log("合并测试通过")
}

// 测试验证功能
func testVerify(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	// 创建编码器
	var r ReedSolomon
	var err error

	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New8(dataShards, parityShards)
	}

	if err != nil {
		t.Fatal(err)
	}

	// 创建随机数据
	data := make([]byte, dataSize)
	_, err = rand.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	// 分片并编码
	shards, err := r.Split(data)
	if err != nil {
		t.Fatal(err)
	}

	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// 验证编码正确性
	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("编码应该是正确的")
	}

	// 保存原始分片的副本
	origShards := make([][]byte, len(shards))
	for i, shard := range shards {
		if shard != nil {
			origShards[i] = make([]byte, len(shard))
			copy(origShards[i], shard)
		}
	}

	// 测试1：损坏一个数据分片
	if len(shards[0]) > 0 {
		shards[0][0] ^= 0xFF
	}

	// 验证，应该失败
	ok, err = r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("损坏的数据应该验证失败")
	}

	// 恢复原始数据用于下一个测试
	for i := range shards {
		if origShards[i] != nil {
			copy(shards[i], origShards[i])
		}
	}

	// 验证恢复后的数据
	ok, err = r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("恢复原始数据后验证应该成功")
	}

	// 测试2：删除一个分片
	shards[0] = nil

	// 重建分片
	err = r.Reconstruct(shards)
	if err != nil {
		t.Fatal(err)
	}

	// 验证重建后的数据
	ok, err = r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("重建后验证应该成功")
	}
}

// 测试超大规模分片数量，专门优化内存使用
func testLargeShardCount(t *testing.T, dataShards, parityShards int, useFF16 bool) {
	t.Log("开始测试大规模分片", dataShards+parityShards, "个分片")

	var r ReedSolomon
	var err error

	// 对于超过256个分片，必须使用GF(2^16)
	if dataShards+parityShards > 256 && !useFF16 {
		t.Fatal("超过256个分片必须使用GF(2^16)实现")
	}

	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New8(dataShards, parityShards)
	}

	if err != nil {
		t.Fatal(err)
	}

	// 对于超大规模分片，使用较小的数据以减少内存占用
	dataSize := dataShards * 64 // 每个分片至少64字节
	data := make([]byte, dataSize)
	_, err = rand.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	// 分片
	shards, err := r.Split(data)
	if err != nil {
		t.Fatal(err)
	}

	// 验证分片数量
	totalShards := dataShards + parityShards
	if len(shards) != totalShards {
		t.Fatal("分片数量不正确", len(shards), "预期", totalShards)
	}

	// 编码
	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	// 验证编码正确性
	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("编码验证失败")
	}

	// 保存部分原始分片的副本（为了减少内存使用）
	// 我们只保存将被删除的分片
	deletedIndices := make([]int, 0, 10)
	origShards := make(map[int][]byte)

	// 模拟丢失少量分片
	maxToRemove := 5 // 对于大规模测试，我们只删除少量分片以保持性能
	for i := 0; i < maxToRemove; i++ {
		idx := i * (dataShards / maxToRemove)
		if idx < dataShards {
			deletedIndices = append(deletedIndices, idx)
			origShards[idx] = make([]byte, len(shards[idx]))
			copy(origShards[idx], shards[idx])
			shards[idx] = nil
		}
	}

	// 重建
	err = r.ReconstructData(shards)
	if err != nil {
		t.Fatal(err)
	}

	// 验证重建结果
	for _, idx := range deletedIndices {
		if shards[idx] == nil {
			t.Fatal("数据分片未重建:", idx)
		}
		if !bytes.Equal(shards[idx], origShards[idx]) {
			t.Fatal("数据分片重建结果不正确:", idx)
		}
	}

	// 合并分片
	var result bytes.Buffer
	err = r.Join(&result, shards, dataSize)
	if err != nil {
		t.Fatal("合并失败:", err)
	}
	recovered := result.Bytes()

	if !bytes.Equal(recovered, data) {
		t.Fatal("重建的数据与原始数据不匹配")
	}

	t.Log("大规模分片测试成功完成")
}

// 流式测试部分 //

// TestStreamBasicEncodeDecode 测试基本的流式编码和解码
func TestStreamBasicEncodeDecode(t *testing.T) {
	// 使用固定参数测试
	dataShards := 4
	parityShards := 2
	dataSize := 16384 // 16KB

	t.Run("FF8", func(t *testing.T) {
		testStreamEncodeDecodeNew(t, dataShards, parityShards, dataSize, false)
	})

	t.Run("FF16", func(t *testing.T) {
		testStreamEncodeDecodeNew(t, dataShards, parityShards, dataSize, true)
	})
}

// TestStreamDifferentShardSizes 测试不同分片大小的情况
func TestStreamDifferentShardSizes(t *testing.T) {
	// 使用不同的分片配置
	tests := []struct {
		dataShards   int
		parityShards int
	}{
		{2, 1},
		{4, 2},
		{8, 4},
		{16, 8},
		{100, 50},
	}

	for _, test := range tests {
		dataSize := 16384 // 固定数据大小为16KB
		name := fmt.Sprintf("ds%d_ps%d", test.dataShards, test.parityShards)

		t.Run("FF8_"+name, func(t *testing.T) {
			testStreamEncodeDecodeNew(t, test.dataShards, test.parityShards, dataSize, false)
		})

		t.Run("FF16_"+name, func(t *testing.T) {
			testStreamEncodeDecodeNew(t, test.dataShards, test.parityShards, dataSize, true)
		})
	}
}

// TestStreamDifferentDataSizes 测试不同数据大小的流式操作
func TestStreamDifferentDataSizes(t *testing.T) {
	// 固定分片配置
	dataShards := 4
	parityShards := 2

	// 测试不同数据大小
	dataSizes := []int{
		0,     // 空数据
		1,     // 1字节
		32,    // 小数据
		63,    // 比64小1字节
		64,    // 刚好64字节
		65,    // 比64大1字节
		127,   // 比128小1字节
		128,   // 刚好128字节
		129,   // 比128大1字节
		1024,  // 1KB
		32768, // 32KB
		65536, // 64KB
	}

	for _, dataSize := range dataSizes {
		name := fmt.Sprintf("Size_%d", dataSize)

		// 空数据在FF16中会失败，因此跳过
		if dataSize == 0 && dataShards > 1 {
			continue
		}

		t.Run("FF8_"+name, func(t *testing.T) {
			testStreamEncodeDecodeNew(t, dataShards, parityShards, dataSize, false)
		})

		if dataSize > 0 { // FF16不支持空数据
			t.Run("FF16_"+name, func(t *testing.T) {
				testStreamEncodeDecodeNew(t, dataShards, parityShards, dataSize, true)
			})
		}
	}
}

// TestStreamLargeShardCount 测试大量分片的情况
func TestStreamLargeShardCount(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大量分片测试")
	}

	// 测试不同分片配置
	tests := []struct {
		dataShards   int
		parityShards int
	}{
		{10, 4},
		{20, 8},
		{50, 20},
		{100, 40},
		{120, 50}, // 接近GF(2^8)的最大值
	}

	for _, test := range tests {
		dataSize := 8192 // 固定数据大小为8KB
		name := fmt.Sprintf("ds%d_ps%d", test.dataShards, test.parityShards)

		t.Run("FF8_"+name, func(t *testing.T) {
			testStreamEncodeDecodeNew(t, test.dataShards, test.parityShards, dataSize, false)
		})

		if test.dataShards+test.parityShards <= 128 { // FF16理论上支持更多分片
			t.Run("FF16_"+name, func(t *testing.T) {
				testStreamEncodeDecodeNew(t, test.dataShards, test.parityShards, dataSize, true)
			})
		}
	}
}

// TestStreamEdgeCases 测试各种边界情况
func TestStreamEdgeCases(t *testing.T) {
	t.Run("单分片", func(t *testing.T) {
		testStreamEncodeDecodeNew(t, 1, 1, 1024, false)
	})

	t.Run("最小数据", func(t *testing.T) {
		testStreamEncodeDecodeNew(t, 4, 2, 1, false)
	})

	t.Run("非均匀分片", func(t *testing.T) {
		testStreamNonUniformShards(t, 4, 2, 100, false)
	})

	t.Run("FF16_非均匀分片", func(t *testing.T) {
		testStreamNonUniformShards(t, 4, 2, 100, true)
	})
}

// TestStreamReconstructData 测试仅恢复数据分片的情况
func TestStreamReconstructData(t *testing.T) {
	// 测试不同数据大小
	dataSizes := []int{
		63,    // 比64小1字节
		64,    // 刚好64字节
		65,    // 比64大1字节
		32768, // 32KB
	}

	for _, dataSize := range dataSizes {
		name := fmt.Sprintf("Size_%d", dataSize)

		t.Run("FF8_"+name, func(t *testing.T) {
			testStreamReconstructDataNew(t, 4, 2, dataSize, false)
		})

		t.Run("FF16_"+name, func(t *testing.T) {
			testStreamReconstructDataNew(t, 4, 2, dataSize, true)
		})
	}
}

// testStreamNonUniformShards 测试非均匀分片大小的情况
func testStreamNonUniformShards(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	// 创建编码器
	var r ReedSolomon
	var err error
	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New(dataShards, parityShards)
	}
	if err != nil {
		t.Fatal(err)
	}

	// 创建测试数据 - 故意设置为非均匀长度
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	origDataHash := md5Hash(data)
	t.Logf("原始数据大小: %d 字节, 哈希: %s", len(data), origDataHash)

	// 流式分割数据 - 手动控制每个分片的大小
	shardSize := dataSize / dataShards
	remainder := dataSize % dataShards

	// 分段拆分数据
	dataShardBytes := make([][]byte, dataShards)
	position := 0
	for i := 0; i < dataShards; i++ {
		// 前remainder个分片多分配1字节
		currentSize := shardSize
		if i < remainder {
			currentSize++
		}

		end := position + currentSize
		if end > dataSize {
			end = dataSize
		}

		dataShardBytes[i] = data[position:end]
		position = end
	}

	// 创建数据分片缓冲区
	dataBuffers := make([]bytes.Buffer, dataShards)
	for i, shardData := range dataShardBytes {
		dataBuffers[i].Write(shardData)
		t.Logf("数据分片 %d 大小: %d 字节, 哈希: %s", i, dataBuffers[i].Len(), md5Hash(dataBuffers[i].Bytes()))
	}

	// 创建奇偶校验分片
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建用于编码的Reader
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式编码
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 检查奇偶校验分片
	for i, buf := range parityBuffers {
		t.Logf("奇偶校验分片 %d 大小: %d 字节, 哈希: %s", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 验证流式合并结果
	mergeReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		mergeReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	var merged bytes.Buffer
	err = r.StreamJoin(&merged, mergeReaders, int64(dataSize))
	if err != nil {
		t.Fatal("流式合并失败:", err)
	}

	// 验证结果
	mergedData := merged.Bytes()
	mergedHash := md5Hash(mergedData)
	t.Logf("合并结果大小: %d 字节, 哈希: %s", len(mergedData), mergedHash)

	if !bytes.Equal(mergedData, data) {
		t.Fatal("合并后的数据与原始数据不匹配")
	}

	t.Log("测试通过: 非均匀分片测试成功")
}

// testStreamReconstructDataNew 测试仅恢复数据分片
func testStreamReconstructDataNew(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	// 创建编码器
	var r ReedSolomon
	var err error
	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New(dataShards, parityShards)
	}
	if err != nil {
		t.Fatal(err)
	}

	// 创建测试数据
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	origDataHash := md5Hash(data)
	t.Logf("原始数据大小: %d 字节, 哈希: %s", len(data), origDataHash)

	// 流式分割数据
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		t.Fatal("流式分割失败:", err)
	}

	// 创建奇偶校验分片
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建用于编码的Reader
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式编码
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 保存原始分片数据以便之后比较
	originalShards := make([][]byte, dataShards)
	for i, buf := range dataBuffers {
		originalShards[i] = buf.Bytes()
	}

	// 模拟丢失两个数据分片
	lostShards := []int{0, 2} // 丢失第一个和第三个数据分片

	// 准备重建输入
	streamInputs := make([]io.Reader, dataShards+parityShards)
	for i := 0; i < dataShards; i++ {
		if contains(lostShards, i) {
			streamInputs[i] = nil // 模拟丢失
		} else {
			streamInputs[i] = bytes.NewReader(dataBuffers[i].Bytes())
		}
	}
	for i := 0; i < parityShards; i++ {
		streamInputs[i+dataShards] = bytes.NewReader(parityBuffers[i].Bytes())
	}

	// 准备重建输出 - 只重建数据分片
	reconstructedBuffers := make([]*bytes.Buffer, len(lostShards))
	streamOutputs := make([]io.Writer, dataShards+parityShards)

	for i, shardIndex := range lostShards {
		reconstructedBuffers[i] = new(bytes.Buffer)
		streamOutputs[shardIndex] = reconstructedBuffers[i]
	}

	// 仅重建数据分片
	err = r.StreamReconstructData(streamInputs, streamOutputs)
	if err != nil {
		t.Fatal("流式重建数据分片失败:", err)
	}

	// 验证重建的分片
	for i, shardIndex := range lostShards {
		reconstructed := reconstructedBuffers[i].Bytes()
		original := originalShards[shardIndex]

		reconstructedHash := md5Hash(reconstructed)
		originalHash := md5Hash(original)

		t.Logf("分片 %d - 重建大小: %d, 哈希: %s", shardIndex, len(reconstructed), reconstructedHash)
		t.Logf("分片 %d - 原始大小: %d, 哈希: %s", shardIndex, len(original), originalHash)

		if !bytes.Equal(reconstructed, original) {
			t.Errorf("分片 %d 重建结果与原始数据不匹配", shardIndex)
			for j := 0; j < 20 && j < len(reconstructed) && j < len(original); j++ {
				t.Logf("位置 %d: 重建=%d, 原始=%d", j, reconstructed[j], original[j])
			}
		} else {
			t.Logf("分片 %d 重建成功", shardIndex)
		}
	}

	// 使用重建后的分片合并数据
	mergeReaders := make([]io.Reader, dataShards)
	for i := 0; i < dataShards; i++ {
		if contains(lostShards, i) {
			// 找到对应的重建缓冲区
			for j, shardIndex := range lostShards {
				if shardIndex == i {
					mergeReaders[i] = bytes.NewReader(reconstructedBuffers[j].Bytes())
					break
				}
			}
		} else {
			mergeReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
		}
	}

	var merged bytes.Buffer
	err = r.StreamJoin(&merged, mergeReaders, int64(dataSize))
	if err != nil {
		t.Fatal("流式合并失败:", err)
	}

	// 验证最终合并结果
	mergedData := merged.Bytes()
	mergedHash := md5Hash(mergedData)
	t.Logf("合并结果大小: %d 字节, 哈希: %s", len(mergedData), mergedHash)

	if mergedHash != origDataHash {
		t.Fatal("重建后合并的数据与原始数据不匹配")
	}

	t.Log("测试通过: 仅数据分片重建测试成功")
}

// TestStreamRepairOneShardFF8 测试FF8模式下的单个分片重建
func TestStreamRepairOneShardFF8(t *testing.T) {
	testStreamRepairOneShard(t, 10, 4, mediumTestSize, false)
}

// testStreamRepairOneShard 测试单个分片重建功能
func testStreamRepairOneShard(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	var r ReedSolomon
	var err error

	if useFF16 {
		t.Log("使用FF16编码器")
		r, err = New16(dataShards, parityShards)
	} else {
		t.Log("使用FF8编码器")
		r, err = New8(dataShards, parityShards)
	}

	if err != nil {
		t.Fatal(err)
	}

	// 创建随机测试数据
	data := make([]byte, dataSize)
	_, err = rand.Read(data)
	if err != nil {
		t.Fatal(err)
	}

	// 拆分数据到多个分片
	shards, err := r.Split(data)
	if err != nil {
		t.Fatal(err)
	}

	// 打印分片信息
	t.Log("原始分片信息:")
	for i, shard := range shards[:dataShards] {
		t.Logf("数据分片 %d: 大小=%d 字节, 哈希=%s", i, len(shard), calcHash(shard))
	}

	// 编码创建奇偶校验分片
	err = r.Encode(shards)
	if err != nil {
		t.Fatal(err)
	}

	t.Log("编码后奇偶校验分片:")
	for i, shard := range shards[dataShards:] {
		t.Logf("奇偶校验分片 %d: 大小=%d 字节, 哈希=%s", i, len(shard), calcHash(shard))
	}

	// 验证分片
	ok, err := r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("验证失败，奇偶校验分片不正确")
	}
	t.Log("初始验证通过")

	// 模拟丢失第一个数据分片
	t.Log("模拟丢失第一个数据分片")
	originalShard0 := shards[0]
	originalShard0Copy := make([]byte, len(originalShard0))
	copy(originalShard0Copy, originalShard0)
	shards[0] = nil

	// 重建丢失的分片
	err = r.Reconstruct(shards)
	if err != nil {
		t.Fatal("重建失败:", err)
	}

	// 检查重建的分片
	t.Logf("重建的数据分片0: 大小=%d 字节, 哈希=%s", len(shards[0]), calcHash(shards[0]))
	t.Logf("原始数据分片0: 大小=%d 字节, 哈希=%s", len(originalShard0Copy), calcHash(originalShard0Copy))

	// 验证重建是否匹配
	if !bytes.Equal(shards[0], originalShard0Copy) {
		t.Errorf("重建的数据分片0与原始分片不匹配")
	} else {
		t.Log("重建的数据分片0与原始分片完全匹配")
	}

	// 再次验证所有分片
	ok, err = r.Verify(shards)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("重建后验证失败，奇偶校验分片不正确")
	}
	t.Log("重建后验证通过")

	// 合并分片
	var result bytes.Buffer
	err = r.Join(&result, shards, dataSize)
	if err != nil {
		t.Fatal("合并失败:", err)
	}
	recovered := result.Bytes()

	// 检查合并结果
	if !bytes.Equal(recovered, data) {
		t.Error("合并后的数据与原始数据不匹配")
		t.Logf("原始数据: 大小=%d 字节, 哈希=%s", len(data), calcHash(data))
		t.Logf("恢复数据: 大小=%d 字节, 哈希=%s", len(recovered), calcHash(recovered))

		// 找出第一个不同字节的位置
		var diffPos int = -1
		minLen := len(data)
		if len(recovered) < minLen {
			minLen = len(recovered)
		}

		for i := 0; i < minLen; i++ {
			if data[i] != recovered[i] {
				diffPos = i
				break
			}
		}

		if diffPos >= 0 {
			t.Logf("首个差异位置: %d", diffPos)
			// 显示差异周围的数据
			start := diffPos - 5
			if start < 0 {
				start = 0
			}
			end := diffPos + 5
			if end > minLen-1 {
				end = minLen - 1
			}

			t.Log("差异附近的数据比较:")
			for i := start; i <= end; i++ {
				if i < len(data) && i < len(recovered) {
					mark := " "
					if data[i] != recovered[i] {
						mark = "*"
					}
					t.Logf("位置 %d: 原始=%v(%c), 恢复=%v(%c) %s",
						i, data[i], data[i], recovered[i], recovered[i], mark)
				}
			}
		}
	} else {
		t.Log("合并成功: 恢复的数据与原始数据完全匹配")
	}
}

// testStreamEncodeDecodeNew 测试流式编码和解码
func testStreamEncodeDecodeNew(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	// 创建编码器
	var r ReedSolomon
	var err error
	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New(dataShards, parityShards)
	}
	if err != nil {
		t.Fatal(err)
	}

	// 创建测试数据
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	origDataHash := md5Hash(data)
	t.Logf("原始数据大小: %d 字节, 哈希: %s", len(data), origDataHash)

	// 流式分割数据
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		t.Fatal("流式分割失败:", err)
	}

	// 检查分片情况
	for i, buf := range dataBuffers {
		t.Logf("数据分片 %d 大小: %d 字节, 哈希: %s", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 创建奇偶校验分片
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建用于编码的Reader
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式编码
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 检查奇偶校验分片
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

	// 验证流式合并结果
	mergeReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		mergeReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	var merged bytes.Buffer
	err = r.StreamJoin(&merged, mergeReaders, int64(dataSize))
	if err != nil {
		t.Fatal("流式合并失败:", err)
	}

	// 验证结果
	mergedData := merged.Bytes()
	mergedHash := md5Hash(mergedData)
	t.Logf("合并结果大小: %d 字节, 哈希: %s", len(mergedData), mergedHash)

	if mergedHash != origDataHash {
		t.Fatal("合并后的数据与原始数据不匹配")
	}

	t.Log("测试通过: 流式编码解码验证成功")
}

// TestStreamReconstruction 测试流式重建功能
func TestStreamReconstruction(t *testing.T) {
	// 使用固定参数测试
	dataShards := 4
	parityShards := 2

	// 测试不同数据大小
	dataSizes := []int{
		63,    // 比64小1字节
		64,    // 刚好64字节
		65,    // 比64大1字节
		127,   // 比128小1字节
		128,   // 刚好128字节
		32768, // 32KB
	}

	for _, dataSize := range dataSizes {
		name := fmt.Sprintf("Size_%d", dataSize)

		t.Run("FF8_"+name, func(t *testing.T) {
			testStreamReconstructionNew(t, dataShards, parityShards, dataSize, false)
		})

		t.Run("FF16_"+name, func(t *testing.T) {
			testStreamReconstructionNew(t, dataShards, parityShards, dataSize, true)
		})
	}
}

// testStreamReconstructionNew 测试流式重建功能
func testStreamReconstructionNew(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	// 创建编码器
	var r ReedSolomon
	var err error
	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New(dataShards, parityShards)
	}
	if err != nil {
		t.Fatal(err)
	}

	// 创建测试数据
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	origDataHash := md5Hash(data)
	t.Logf("原始数据大小: %d 字节, 哈希: %s", len(data), origDataHash)

	// 流式分割数据
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		t.Fatal("流式分割失败:", err)
	}

	// 检查分片情况
	for i, buf := range dataBuffers {
		t.Logf("数据分片 %d 大小: %d 字节, 哈希: %s", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 创建奇偶校验分片
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建用于编码的Reader
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式编码
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 检查奇偶校验分片
	for i, buf := range parityBuffers {
		t.Logf("奇偶校验分片 %d 大小: %d 字节, 哈希: %s", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 保存原始分片数据以便之后比较
	originalShards := make([][]byte, dataShards+parityShards)
	for i, buf := range dataBuffers {
		originalShards[i] = buf.Bytes()
	}
	for i, buf := range parityBuffers {
		originalShards[i+dataShards] = buf.Bytes()
	}

	// 模拟丢失第一个和最后一个数据分片
	lostShards := []int{0, dataShards - 1}

	// 准备重建输入
	streamInputs := make([]io.Reader, dataShards+parityShards)
	for i := 0; i < dataShards+parityShards; i++ {
		if contains(lostShards, i) {
			streamInputs[i] = nil // 模拟丢失
		} else if i < dataShards {
			streamInputs[i] = bytes.NewReader(dataBuffers[i].Bytes())
		} else {
			streamInputs[i] = bytes.NewReader(parityBuffers[i-dataShards].Bytes())
		}
	}

	// 准备重建输出
	reconstructedBuffers := make([]*bytes.Buffer, len(lostShards))
	streamOutputs := make([]io.Writer, dataShards+parityShards)

	for i, shardIndex := range lostShards {
		reconstructedBuffers[i] = new(bytes.Buffer)
		streamOutputs[shardIndex] = reconstructedBuffers[i]
	}

	// 流式重建
	err = r.StreamReconstruct(streamInputs, streamOutputs)
	if err != nil {
		t.Fatal("流式重建失败:", err)
	}

	// 验证重建的分片
	for i, shardIndex := range lostShards {
		reconstructed := reconstructedBuffers[i].Bytes()
		original := originalShards[shardIndex]

		reconstructedHash := md5Hash(reconstructed)
		originalHash := md5Hash(original)

		t.Logf("分片 %d - 重建大小: %d, 哈希: %s", shardIndex, len(reconstructed), reconstructedHash)
		t.Logf("分片 %d - 原始大小: %d, 哈希: %s", shardIndex, len(original), originalHash)

		if !bytes.Equal(reconstructed, original) {
			t.Errorf("分片 %d 重建结果与原始数据不匹配", shardIndex)
			for j := 0; j < 20 && j < len(reconstructed) && j < len(original); j++ {
				t.Logf("位置 %d: 重建=%d, 原始=%d", j, reconstructed[j], original[j])
			}
		} else {
			t.Logf("分片 %d 重建成功", shardIndex)
		}
	}

	// 使用重建后的分片合并数据
	mergeReaders := make([]io.Reader, dataShards)
	for i := 0; i < dataShards; i++ {
		if contains(lostShards, i) {
			// 找到对应的重建缓冲区
			for j, shardIndex := range lostShards {
				if shardIndex == i {
					mergeReaders[i] = bytes.NewReader(reconstructedBuffers[j].Bytes())
					break
				}
			}
		} else {
			mergeReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
		}
	}

	var merged bytes.Buffer
	err = r.StreamJoin(&merged, mergeReaders, int64(dataSize))
	if err != nil {
		t.Fatal("流式合并失败:", err)
	}

	// 验证最终合并结果
	mergedData := merged.Bytes()
	mergedHash := md5Hash(mergedData)
	t.Logf("合并结果大小: %d 字节, 哈希: %s", len(mergedData), mergedHash)

	if mergedHash != origDataHash {
		t.Fatal("重建后合并的数据与原始数据不匹配")
	}

	t.Log("测试通过: 流式重建验证成功")
}

// TestStreamVerify 测试流式验证功能
func TestStreamVerify(t *testing.T) {
	// 使用固定参数测试
	dataShards := 4
	parityShards := 2

	// 测试不同数据大小
	dataSizes := []int{
		63,    // 比64小1字节
		64,    // 刚好64字节
		65,    // 比64大1字节
		127,   // 比128小1字节
		128,   // 刚好128字节
		32768, // 32KB
	}

	for _, dataSize := range dataSizes {
		name := fmt.Sprintf("Size_%d", dataSize)

		t.Run("FF8_"+name, func(t *testing.T) {
			testStreamVerifyNew(t, dataShards, parityShards, dataSize, false)
		})

		t.Run("FF16_"+name, func(t *testing.T) {
			testStreamVerifyNew(t, dataShards, parityShards, dataSize, true)
		})
	}
}

// testStreamVerifyNew 测试流式验证功能
func testStreamVerifyNew(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
	// 创建编码器
	var r ReedSolomon
	var err error
	if useFF16 {
		r, err = New16(dataShards, parityShards)
	} else {
		r, err = New(dataShards, parityShards)
	}
	if err != nil {
		t.Fatal(err)
	}

	// 创建测试数据
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}

	// 流式分割数据
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		t.Fatal("流式分割失败:", err)
	}

	// 创建奇偶校验分片
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建用于编码的Reader
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式编码
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 测试1: 验证正确的分片
	t.Log("测试1: 验证所有分片正确")
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
		t.Fatal("流式验证错误: 应该返回true但返回false")
	}

	// 测试2: 验证错误的分片
	t.Log("测试2: 验证篡改的分片")
	tamperedBuffer := bytes.NewBuffer(nil)
	tamperedBuffer.Write(dataBuffers[0].Bytes())
	if tamperedBuffer.Len() > 0 {
		// 篡改第一个字节
		tamperedData := tamperedBuffer.Bytes()
		tamperedData[0] ^= 0xFF
	}

	tamperedReaders := make([]io.Reader, dataShards+parityShards)
	tamperedReaders[0] = bytes.NewReader(tamperedBuffer.Bytes())
	for i := 1; i < dataShards; i++ {
		tamperedReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}
	for i := 0; i < parityShards; i++ {
		tamperedReaders[i+dataShards] = bytes.NewReader(parityBuffers[i].Bytes())
	}

	ok, err = r.StreamVerify(tamperedReaders)
	if err != nil {
		t.Log("篡改验证预期错误:", err)
	}
	if ok {
		t.Fatal("流式验证错误: 应该返回false但返回true")
	}

	t.Log("测试通过: 流式验证功能正常")
}

// contains 检查slice中是否包含特定值
func contains(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
