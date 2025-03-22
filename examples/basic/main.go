package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"log"

	rs "github.com/bpfs/reedsolomon16"
)

// 计算MD5哈希值
func md5Hash(data []byte) string {
	hash := md5.Sum(data)
	return fmt.Sprintf("%x", hash[:])
}

func main() {
	fmt.Println("Reed-Solomon 基本示例")
	fmt.Println("===================")

	// 创建一个具有4个数据分片和2个奇偶校验分片的编码器
	// 这与测试用例中常用的配置一致
	dataShards := 4
	parityShards := 2
	enc, err := rs.New(dataShards, parityShards)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("编码器配置: 数据分片=%d, 校验分片=%d, 总分片=%d\n\n",
		enc.DataShards(), enc.ParityShards(), enc.TotalShards())

	// 使用确定性数据，与测试用例一致
	dataSize := 256 // 使用小数据进行演示
	originalData := make([]byte, dataSize)
	for i := range originalData {
		originalData[i] = byte(i % 256)
	}

	originalHash := md5Hash(originalData)
	fmt.Printf("原始数据大小: %d 字节, 哈希: %s\n", len(originalData), originalHash)

	// 示例1: 基本编码和重建
	fmt.Println("\n示例1: 基本编码和重建")
	fmt.Println("---------------------")
	basicEncodeDecode(enc, originalData, originalHash)

	// 示例2: 更多分片丢失场景
	fmt.Println("\n示例2: 更多分片丢失场景")
	fmt.Println("----------------------")
	advancedReconstruction(enc, originalData)

	// 示例3: 数据校验
	fmt.Println("\n示例3: 数据校验")
	fmt.Println("-------------")
	dataVerification(enc, originalData)
}

// 基本编码和重建示例
func basicEncodeDecode(enc rs.ReedSolomon, originalData []byte, originalHash string) {
	// 分割数据
	shards, err := enc.Split(originalData)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("数据已分割为 %d 个数据分片\n", enc.DataShards())

	// 显示分片信息
	for i, shard := range shards[:enc.DataShards()] {
		fmt.Printf("数据分片 %d: %d 字节, 哈希: %s\n", i, len(shard), md5Hash(shard))
	}

	// 编码（生成校验分片）
	err = enc.Encode(shards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("已生成校验分片")

	// 显示校验分片信息
	for i := enc.DataShards(); i < enc.TotalShards(); i++ {
		fmt.Printf("校验分片 %d: %d 字节, 哈希: %s\n", i, len(shards[i]), md5Hash(shards[i]))
	}

	// 验证所有分片
	ok, err := enc.Verify(shards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("编码后验证结果: %v\n", ok)

	// 保存原始分片以便后续比较
	originalShards := make([][]byte, len(shards))
	for i, shard := range shards {
		originalShards[i] = make([]byte, len(shard))
		copy(originalShards[i], shard)
	}

	// 模拟丢失一个分片
	fmt.Println("\n模拟丢失一个数据分片和一个校验分片")
	lostDataShard := 1   // 丢失的数据分片
	lostParityShard := 4 // 丢失的校验分片

	shards[lostDataShard] = nil
	shards[lostParityShard] = nil

	// 重建丢失的分片
	err = enc.Reconstruct(shards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("已重建丢失的分片")

	// 验证重建结果
	fmt.Printf("数据分片 %d - 重建成功: %v\n",
		lostDataShard,
		bytes.Equal(shards[lostDataShard], originalShards[lostDataShard]))

	fmt.Printf("校验分片 %d - 重建成功: %v\n",
		lostParityShard,
		bytes.Equal(shards[lostParityShard], originalShards[lostParityShard]))

	// 再次验证
	ok, err = enc.Verify(shards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("重建后验证结果: %v\n", ok)

	// 重建原始数据
	var buf bytes.Buffer
	err = enc.Join(&buf, shards, len(originalData))
	if err != nil {
		log.Fatal(err)
	}

	// 验证数据完整性
	recoveredData := buf.Bytes()
	recoveredHash := md5Hash(recoveredData)
	fmt.Printf("原始数据哈希: %s\n", originalHash)
	fmt.Printf("恢复数据哈希: %s\n", recoveredHash)
	fmt.Printf("数据恢复成功: %v\n", originalHash == recoveredHash)
}

// 高级重建场景示例
func advancedReconstruction(enc rs.ReedSolomon, originalData []byte) {
	// 场景1: 丢失两个数据分片
	fmt.Println("\n场景1: 丢失两个数据分片")
	shards, _ := enc.Split(originalData)
	// 添加校验分片
	shards[4] = make([]byte, len(shards[0]))
	shards[5] = make([]byte, len(shards[0]))
	enc.Encode(shards)

	// 模拟丢失数据分片
	fmt.Println("丢失数据分片 0 和 1")
	shards[0] = nil
	shards[1] = nil

	// 重建
	err := enc.Reconstruct(shards)
	if err != nil {
		fmt.Printf("重建失败: %v\n", err)
	} else {
		fmt.Println("重建成功")
		// 恢复原始数据并验证
		var buf bytes.Buffer
		enc.Join(&buf, shards, len(originalData))
		reconstructed := buf.Bytes()
		fmt.Printf("数据完整性: %v\n", bytes.Equal(reconstructed, originalData))
	}

	// 场景2: 丢失两个校验分片
	fmt.Println("\n场景2: 丢失两个校验分片")
	shards, _ = enc.Split(originalData)
	// 添加校验分片
	shards[4] = make([]byte, len(shards[0]))
	shards[5] = make([]byte, len(shards[0]))
	enc.Encode(shards)

	// 模拟丢失校验分片
	fmt.Println("丢失校验分片 4 和 5")
	shards[4] = nil
	shards[5] = nil

	// 重建
	err = enc.Reconstruct(shards)
	if err != nil {
		fmt.Printf("重建失败: %v\n", err)
	} else {
		fmt.Println("重建成功")
		// 验证
		ok, _ := enc.Verify(shards)
		fmt.Printf("验证结果: %v\n", ok)
	}

	// 场景3: 数据和校验分片都丢失
	fmt.Println("\n场景3: 丢失一个数据分片和一个校验分片")
	shards, _ = enc.Split(originalData)
	// 添加校验分片
	shards[4] = make([]byte, len(shards[0]))
	shards[5] = make([]byte, len(shards[0]))
	enc.Encode(shards)

	// 模拟丢失
	fmt.Println("丢失数据分片 2 和校验分片 5")
	shards[2] = nil
	shards[5] = nil

	// 重建
	err = enc.Reconstruct(shards)
	if err != nil {
		fmt.Printf("重建失败: %v\n", err)
	} else {
		fmt.Println("重建成功")
		// 恢复原始数据并验证
		var buf bytes.Buffer
		enc.Join(&buf, shards, len(originalData))
		reconstructed := buf.Bytes()
		fmt.Printf("数据完整性: %v\n", bytes.Equal(reconstructed, originalData))
	}

	// 场景4: 超出容错能力
	fmt.Println("\n场景4: 丢失超过容错能力的分片")
	shards, _ = enc.Split(originalData)
	// 添加校验分片
	shards[4] = make([]byte, len(shards[0]))
	shards[5] = make([]byte, len(shards[0]))
	enc.Encode(shards)

	// 模拟丢失
	fmt.Println("丢失数据分片 0、1 和 2（超过容错能力）")
	shards[0] = nil
	shards[1] = nil
	shards[2] = nil

	// 尝试重建
	err = enc.Reconstruct(shards)
	if err != nil {
		fmt.Printf("符合预期的重建失败: %v\n", err)
	} else {
		fmt.Println("意外重建成功")
	}
}

// 数据校验示例
func dataVerification(enc rs.ReedSolomon, originalData []byte) {
	// 首先获取分片大小的倍数要求
	multiple := enc.ShardSizeMultiple()

	// 保存原始数据的长度和哈希，以及副本
	originalLen := len(originalData)
	originalDataHash := md5Hash(originalData)
	// 创建原始数据的副本，用于后续比较
	initialData := make([]byte, originalLen)
	copy(initialData, originalData)

	fmt.Printf("原始数据长度: %d, 哈希值: %s\n", originalLen, originalDataHash)

	// 确保数据大小是分片大小的倍数
	dataLen := len(originalData)
	var paddedData []byte
	if dataLen%multiple != 0 {
		paddedSize := dataLen + (multiple - dataLen%multiple)
		paddedData = make([]byte, paddedSize)
		copy(paddedData, originalData)
		originalData = paddedData
		fmt.Printf("填充后数据长度: %d (填充了 %d 字节)\n", len(originalData), len(originalData)-originalLen)
	}

	// 重新分片
	shards, _ := enc.Split(originalData)
	enc.Encode(shards)

	// 保存未修改的原始分片
	originalShards := make([][]byte, len(shards))
	for i, shard := range shards {
		originalShards[i] = make([]byte, len(shard))
		copy(originalShards[i], shard)
	}

	// 1. 验证正确的分片
	fmt.Println("1. 验证正确的分片")
	ok, err := enc.Verify(shards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("验证结果: %v\n", ok)

	// 2. 修改一个数据分片
	fmt.Println("\n2. 修改一个数据分片后验证")
	// 篡改数据
	if len(shards[0]) > 10 {
		fmt.Printf("将分片0的第10字节从 %d 修改为 ", shards[0][10])
		shards[0][10] ^= 0xFF // 翻转一个字节的比特
		fmt.Printf("%d\n", shards[0][10])
	}

	ok, err = enc.Verify(shards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("验证结果: %v (应为false)\n", ok)

	// 3. 修复被篡改的数据
	fmt.Println("\n3. 尝试修复被篡改的数据")
	// 标记被篡改的分片为丢失
	shards[0] = nil

	// 重建
	err = enc.Reconstruct(shards)
	if err != nil {
		fmt.Printf("修复失败: %v\n", err)
	} else {
		fmt.Println("修复成功")
		// 比较修复的数据与原始未篡改的分片
		fmt.Printf("修复的分片与原始分片比较: %v\n",
			bytes.Equal(shards[0], originalShards[0]))

		// 验证所有分片
		ok, _ := enc.Verify(shards)
		fmt.Printf("验证结果: %v\n", ok)

		// 恢复原始数据并验证
		var buf bytes.Buffer
		err = enc.Join(&buf, shards, len(originalData))
		if err != nil {
			log.Printf("Join失败: %v", err)
		} else {
			reconstructed := buf.Bytes()

			// 输出长度信息
			fmt.Printf("重建数据长度: %d, 原始数据长度: %d\n",
				len(reconstructed), originalLen)

			// 比较字节数组 - 使用初始未修改的数据进行比较
			byteEquals := bytes.Equal(reconstructed[:originalLen], initialData)
			fmt.Printf("与初始未修改数据比较: %v\n", byteEquals)

			// 计算哈希值比较数据内容
			reconstructedHash := md5Hash(reconstructed[:originalLen])
			initialDataHash := md5Hash(initialData)
			fmt.Printf("重建数据哈希: %s\n初始数据哈希: %s\n哈希值相等: %v\n",
				reconstructedHash, initialDataHash, reconstructedHash == initialDataHash)
		}
	}
}
