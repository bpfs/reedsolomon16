package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"log"

	rs "github.com/bpfs/reedsolomon16"
)

// 计算MD5哈希值
func md5Hash(data []byte) string {
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash[:])
}

func main() {
	// 设置参数 - 与测试用例相同的配置
	dataShards := 4
	parityShards := 2
	dataSize := 128 * 1024 // 128KB 数据大小

	// 创建编码器
	enc, err := rs.New(dataShards, parityShards)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("创建 Reed-Solomon 编码器: 数据分片=%d, 校验分片=%d\n", dataShards, parityShards)

	// 创建测试数据 - 与测试用例使用相同的数据模式
	data := make([]byte, dataSize)
	for i := range data {
		data[i] = byte(i % 256) // 使用确定性数据而非随机数
	}
	originalHash := md5Hash(data)
	fmt.Printf("原始数据: %d 字节, 哈希: %s\n", len(data), originalHash)

	// 1. 流式分割数据
	fmt.Println("\n1. 流式分割数据...")
	dataBuffers := make([]bytes.Buffer, dataShards)
	dataWriters := make([]io.Writer, dataShards)
	for i := range dataBuffers {
		dataWriters[i] = &dataBuffers[i]
	}

	// 使用流式分割
	err = enc.StreamSplit(bytes.NewReader(data), dataWriters, int64(dataSize))
	if err != nil {
		log.Fatal("流式分割失败:", err)
	}

	// 打印分片信息
	for i, buf := range dataBuffers {
		fmt.Printf("数据分片 %d: %d 字节, 哈希: %s\n", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 2. 流式编码生成校验分片
	fmt.Println("\n2. 流式编码生成校验分片...")
	parityBuffers := make([]bytes.Buffer, parityShards)
	parityWriters := make([]io.Writer, parityShards)
	for i := range parityBuffers {
		parityWriters[i] = &parityBuffers[i]
	}

	// 创建数据分片读取器
	dataReaders := make([]io.Reader, dataShards)
	for i := range dataBuffers {
		dataReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}

	// 流式编码
	err = enc.StreamEncode(dataReaders, parityWriters)
	if err != nil {
		log.Fatal("流式编码失败:", err)
	}

	// 打印校验分片信息
	for i, buf := range parityBuffers {
		fmt.Printf("校验分片 %d: %d 字节, 哈希: %s\n", i, buf.Len(), md5Hash(buf.Bytes()))
	}

	// 3. 流式验证分片
	fmt.Println("\n3. 流式验证分片...")
	allReaders := make([]io.Reader, dataShards+parityShards)
	for i := 0; i < dataShards; i++ {
		allReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
	}
	for i := 0; i < parityShards; i++ {
		allReaders[i+dataShards] = bytes.NewReader(parityBuffers[i].Bytes())
	}

	ok, err := enc.StreamVerify(allReaders)
	if err != nil {
		log.Fatal("流式验证失败:", err)
	}
	fmt.Printf("验证结果: %v\n", ok)

	// 保存原始分片以便后续比较
	originalShards := make([][]byte, dataShards+parityShards)
	for i, buf := range dataBuffers {
		originalShards[i] = make([]byte, buf.Len())
		copy(originalShards[i], buf.Bytes())
	}
	for i, buf := range parityBuffers {
		originalShards[i+dataShards] = make([]byte, buf.Len())
		copy(originalShards[i+dataShards], buf.Bytes())
	}

	// 4. 模拟丢失分片
	fmt.Println("\n4. 模拟丢失分片...")
	lostShards := []int{0, 2} // 丢失第1和第3个数据分片
	fmt.Printf("丢失的分片: %v\n", lostShards)

	// 5. 流式重建丢失的分片
	fmt.Println("\n5. 流式重建丢失的分片...")
	streamInputs := make([]io.Reader, dataShards+parityShards)
	streamOutputs := make([]io.Writer, dataShards+parityShards)

	// 设置可用分片作为输入
	for i := 0; i < dataShards+parityShards; i++ {
		if i == lostShards[0] || i == lostShards[1] {
			streamInputs[i] = nil // 丢失的分片设为nil
		} else if i < dataShards {
			streamInputs[i] = bytes.NewReader(dataBuffers[i].Bytes())
		} else {
			streamInputs[i] = bytes.NewReader(parityBuffers[i-dataShards].Bytes())
		}
	}

	// 准备重建的输出缓冲区
	reconstructedBuffers := make([]*bytes.Buffer, len(lostShards))
	for i, idx := range lostShards {
		reconstructedBuffers[i] = new(bytes.Buffer)
		streamOutputs[idx] = reconstructedBuffers[i]
	}

	// 执行流式重建
	err = enc.StreamReconstruct(streamInputs, streamOutputs)
	if err != nil {
		log.Fatal("流式重建失败:", err)
	}

	// 验证重建结果
	fmt.Println("\n6. 验证重建的分片...")
	for i, idx := range lostShards {
		rebuilt := reconstructedBuffers[i].Bytes()
		original := originalShards[idx]
		matches := bytes.Equal(rebuilt, original)

		fmt.Printf("分片 %d - 原始哈希: %s\n", idx, md5Hash(original))
		fmt.Printf("分片 %d - 重建哈希: %s\n", idx, md5Hash(rebuilt))
		fmt.Printf("分片 %d - 重建%s\n", idx, map[bool]string{true: "成功", false: "失败"}[matches])
	}

	// 7. 流式合并重建后的数据
	fmt.Println("\n7. 流式合并重建后的数据...")

	// 准备合并输入
	mergeReaders := make([]io.Reader, dataShards)
	for i := 0; i < dataShards; i++ {
		if i == lostShards[0] {
			mergeReaders[i] = bytes.NewReader(reconstructedBuffers[0].Bytes())
		} else if i == lostShards[1] {
			mergeReaders[i] = bytes.NewReader(reconstructedBuffers[1].Bytes())
		} else {
			mergeReaders[i] = bytes.NewReader(dataBuffers[i].Bytes())
		}
	}

	// 执行流式合并
	var recovered bytes.Buffer
	err = enc.StreamJoin(&recovered, mergeReaders, int64(dataSize))
	if err != nil {
		log.Fatal("流式合并失败:", err)
	}

	// 验证恢复的数据
	recoveredData := recovered.Bytes()
	recoveredHash := md5Hash(recoveredData)

	fmt.Printf("原始数据哈希: %s\n", originalHash)
	fmt.Printf("恢复数据哈希: %s\n", recoveredHash)
	fmt.Printf("数据恢复%s\n", map[bool]string{true: "成功", false: "失败"}[originalHash == recoveredHash])

	// 如果哈希不匹配，显示差异
	if originalHash != recoveredHash {
		fmt.Println("\n发现数据差异:")
		// 找出第一个不同的字节
		for i := 0; i < len(data) && i < len(recoveredData); i++ {
			if data[i] != recoveredData[i] {
				fmt.Printf("位置 %d: 原始=%d, 恢复=%d\n", i, data[i], recoveredData[i])
				break
			}
		}
	}

	fmt.Println("\nReed-Solomon 流式接口示例执行完成!")
}
