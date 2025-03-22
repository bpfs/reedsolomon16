package reedsolomon

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"testing"
)

// 测试不同对齐大小的数据在流式处理中的行为
func TestAlignmentStreamReconstruction(t *testing.T) {
	// 测试多种大小，特别是接近64字节边界的大小
	testSizes := []int{
		63,    // 比64小1字节
		64,    // 刚好64字节
		65,    // 比64大1字节
		127,   // 比128小1字节
		128,   // 刚好128字节
		129,   // 比128大1字节
		32768, // 32KB - 典型测试
	}

	// 固定分片配置
	dataShards := 4
	parityShards := 2

	for _, dataSize := range testSizes {
		t.Run(fmt.Sprintf("%d", dataSize), func(t *testing.T) {
			testAlignmentReconstruction(t, dataShards, parityShards, dataSize, false)
		})
	}
}

// 测试特定大小数据的流式重建
func testAlignmentReconstruction(t *testing.T, dataShards, parityShards, dataSize int, useFF16 bool) {
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

	// 创建固定模式的测试数据
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	originalHash := calcMD5Hash(data)
	t.Logf("原始数据: 大小=%d字节, 哈希=%s", dataSize, originalHash)

	// 创建数据分片缓冲区
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	// 拆分数据到各个分片
	err = r.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		t.Fatal("流式拆分失败:", err)
	}

	// 打印数据分片大小和哈希
	t.Log("数据分片信息:")
	for i, buf := range dataBuffers {
		t.Logf("数据分片 %d: 大小=%d 字节, 哈希=%s", i, buf.Len(), calcMD5Hash(buf.Bytes()))
	}

	// 创建奇偶校验分片缓冲区
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建数据分片Reader
	dataReaders := make([]io.Reader, dataShards)
	for i, buf := range dataBuffers {
		dataReaders[i] = bytes.NewReader(buf.Bytes())
	}

	// 流式编码
	err = r.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		t.Fatal("流式编码失败:", err)
	}

	// 打印校验分片大小和哈希
	t.Log("奇偶校验分片信息:")
	for i, buf := range parityBuffers {
		t.Logf("奇偶校验分片 %d: 大小=%d 字节, 哈希=%s", i, buf.Len(), calcMD5Hash(buf.Bytes()))
	}

	// 验证分片内容
	allReaders := make([]io.Reader, dataShards+parityShards)
	for i, buf := range dataBuffers {
		allReaders[i] = bytes.NewReader(buf.Bytes())
	}
	for i, buf := range parityBuffers {
		allReaders[dataShards+i] = bytes.NewReader(buf.Bytes())
	}

	ok, err := r.StreamVerify(allReaders)
	if err != nil {
		t.Fatal("流式验证失败:", err)
	}
	if !ok {
		t.Fatal("验证失败，奇偶校验分片不正确")
	}
	t.Log("验证通过: 分片编码正确")

	// 测试重建场景1: 删除一个数据分片
	t.Log("场景1: 删除一个数据分片并重建")

	// 重建输入输出准备
	reconInputs := make([]io.Reader, dataShards+parityShards)
	reconOutputs := make([]io.Writer, dataShards+parityShards)
	reconBuffers := make([]*bytes.Buffer, dataShards+parityShards)

	// 复制所有分片，但删除第一个数据分片
	for i := 0; i < dataShards+parityShards; i++ {
		if i == 0 {
			reconInputs[i] = nil // 删除第一个数据分片
			reconBuffers[i] = new(bytes.Buffer)
			reconOutputs[i] = reconBuffers[i]
		} else if i < dataShards {
			reconInputs[i] = bytes.NewReader(dataBuffers[i].Bytes())
		} else {
			reconInputs[i] = bytes.NewReader(parityBuffers[i-dataShards].Bytes())
		}
	}

	// 执行重建
	err = r.StreamReconstruct(reconInputs, reconOutputs)
	if err != nil {
		t.Fatal("流式重建失败:", err)
	}

	// 验证重建结果
	rebuiltHash := calcMD5Hash(reconBuffers[0].Bytes())
	originalHash0 := calcMD5Hash(dataBuffers[0].Bytes())
	t.Logf("重建的数据分片0: 大小=%d字节, 哈希=%s", reconBuffers[0].Len(), rebuiltHash)
	t.Logf("原始数据分片0: 大小=%d字节, 哈希=%s", dataBuffers[0].Len(), originalHash0)

	// 比较哈希值
	if rebuiltHash != originalHash0 {
		t.Errorf("重建的数据分片0与原始分片不匹配")
		// 打印字节级别差异
		compareBuffers(t, reconBuffers[0].Bytes(), dataBuffers[0].Bytes())
	} else {
		t.Log("重建的数据分片0与原始分片匹配")
	}

	// 验证重建后的完整分片
	verifyReaders := make([]io.Reader, dataShards+parityShards)
	verifyReaders[0] = bytes.NewReader(reconBuffers[0].Bytes())
	for i := 1; i < dataShards+parityShards; i++ {
		if i < dataShards {
			verifyReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
		} else {
			verifyReaders[i] = bytes.NewReader(parityBuffers[i-dataShards].Bytes())
		}
	}

	ok, err = r.StreamVerify(verifyReaders)
	if err != nil {
		t.Fatal("重建后验证失败:", err)
	}
	if !ok {
		t.Fatal("重建后验证不通过")
	}
	t.Log("验证通过: 重建后的分片正确")

	// 测试合并场景
	t.Log("合并测试: 使用重建的分片合并数据")

	// 准备合并用的Reader
	mergeReaders := make([]io.Reader, dataShards)
	mergeReaders[0] = bytes.NewReader(reconBuffers[0].Bytes())
	for i := 1; i < dataShards; i++ {
		mergeReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 执行合并
	var result bytes.Buffer
	err = r.StreamJoin(&result, mergeReaders, int64(dataSize))
	if err != nil {
		t.Fatal("流式合并失败:", err)
	}

	// 验证合并结果
	resultHash := calcMD5Hash(result.Bytes())
	t.Logf("合并结果: 大小=%d字节, 哈希=%s", result.Len(), resultHash)
	t.Logf("原始数据: 大小=%d字节, 哈希=%s", len(data), originalHash)

	if resultHash != originalHash {
		t.Error("合并后的数据与原始数据不匹配")
		compareBuffers(t, result.Bytes(), data)
	} else {
		t.Log("合并后的数据与原始数据匹配")
	}
}

// 计算MD5哈希值
func calcMD5Hash(data []byte) string {
	hash := md5.Sum(data)
	return hex.EncodeToString(hash[:])
}

// 比较两个缓冲区的内容并打印差异
func compareBuffers(t *testing.T, buf1, buf2 []byte) {
	minLen := len(buf1)
	if len(buf2) < minLen {
		minLen = len(buf2)
	}

	// 找到第一个不同位置
	diffPos := -1
	for i := 0; i < minLen; i++ {
		if buf1[i] != buf2[i] {
			diffPos = i
			break
		}
	}

	if diffPos >= 0 {
		t.Logf("首个差异位置: %d", diffPos)

		// 打印差异周围的内容
		start := diffPos - 5
		if start < 0 {
			start = 0
		}
		end := diffPos + 5
		if end > minLen-1 {
			end = minLen - 1
		}

		t.Log("差异附近的内容比较:")
		for i := start; i <= end; i++ {
			mark := " "
			if buf1[i] != buf2[i] {
				mark = "*"
			}
			t.Logf("位置 %d: buf1=%v, buf2=%v %s", i, buf1[i], buf2[i], mark)
		}
	}

	if len(buf1) != len(buf2) {
		t.Logf("长度不同: buf1=%d字节, buf2=%d字节", len(buf1), len(buf2))
	}
}
