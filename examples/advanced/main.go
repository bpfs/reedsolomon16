package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"time"

	rs "github.com/bpfs/reedsolomon16"
)

func main() {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "rs_advanced_test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("Reed-Solomon 高级选项示例")
	fmt.Println("========================")

	// 获取CPU核心数，用于设置并发参数
	cpuCores := runtime.NumCPU()
	fmt.Printf("检测到 %d 个CPU核心\n", cpuCores)

	// 创建基本编码器
	enc, err := rs.New(10, 3)
	if err != nil {
		log.Fatal(err)
	}

	// 设置并发级别
	enc = enc.WithConcurrency(cpuCores)

	fmt.Println("已创建编码器并启用并发处理")

	// 创建大块数据用于测试
	dataSize := 50 * 1024 * 1024 // 50MB (调小以适应不同性能的机器)
	shards := make([][]byte, 13) // 10个数据分片 + 3个校验分片

	fmt.Printf("分配 %d MB 的测试数据...\n", (dataSize*10)/1024/1024)

	// 填充随机数据（使用伪随机数据加速测试）
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < 10; i++ {
		shards[i] = make([]byte, dataSize)
		// 使用随机填充以更好地模拟真实数据
		rand.Read(shards[i])
		fmt.Printf("已生成数据分片 %d (%d MB)\n", i, len(shards[i])/1024/1024)
	}

	// 为校验分片分配内存
	for i := 10; i < 13; i++ {
		shards[i] = make([]byte, dataSize)
		fmt.Printf("已分配校验分片 %d (%d MB)\n", i, len(shards[i])/1024/1024)
	}

	// 执行基准测试函数
	fmt.Println("\n开始性能基准测试...")
	performBenchmarks(enc, shards, dataSize)

	// 测试文件持久化和读取
	fmt.Println("\n测试分片文件持久化...")
	testFilePersistence(enc, shards, tmpDir)
}

// 执行一系列性能基准测试
func performBenchmarks(enc rs.ReedSolomon, shards [][]byte, dataSize int) {
	// 测试编码性能
	fmt.Println("\n1. 编码性能测试")
	start := time.Now()
	err := enc.Encode(shards)
	if err != nil {
		log.Fatal(err)
	}
	encodeTime := time.Since(start)
	fmt.Printf("编码完成，用时: %v\n", encodeTime)
	fmt.Printf("编码速度: %.2f MB/s\n", float64(dataSize*10)/encodeTime.Seconds()/1024/1024)

	// 测试验证性能
	fmt.Println("\n2. 验证性能测试")
	start = time.Now()
	ok, err := enc.Verify(shards)
	if err != nil {
		log.Fatal(err)
	}
	verifyTime := time.Since(start)
	fmt.Printf("验证完成，用时: %v\n", verifyTime)
	fmt.Printf("验证速度: %.2f MB/s\n", float64(dataSize*13)/verifyTime.Seconds()/1024/1024)
	fmt.Printf("验证结果: %v\n", ok)

	// 模拟多种故障场景
	fmt.Println("\n3. 多种故障场景测试")

	// 场景1：丢失数据分片
	fmt.Println("\n3.1 丢失数据分片")
	testShards := make([][]byte, len(shards))
	copy(testShards, shards)

	testShards[2] = nil // 丢失数据分片
	testShards[5] = nil // 丢失数据分片

	start = time.Now()
	err = enc.Reconstruct(testShards)
	if err != nil {
		log.Fatal(err)
	}
	reconstructTime := time.Since(start)
	fmt.Printf("重建完成，用时: %v\n", reconstructTime)
	fmt.Printf("重建速度: %.2f MB/s\n", float64(dataSize*2)/reconstructTime.Seconds()/1024/1024)

	ok, err = enc.Verify(testShards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("重建后验证结果: %v\n", ok)

	// 场景2：丢失校验分片
	fmt.Println("\n3.2 丢失校验分片")
	testShards = make([][]byte, len(shards))
	copy(testShards, shards)

	testShards[10] = nil // 丢失校验分片
	testShards[11] = nil // 丢失校验分片
	testShards[12] = nil // 丢失校验分片

	start = time.Now()
	err = enc.Reconstruct(testShards)
	if err != nil {
		log.Fatal(err)
	}
	reconstructTime = time.Since(start)
	fmt.Printf("重建完成，用时: %v\n", reconstructTime)
	fmt.Printf("重建速度: %.2f MB/s\n", float64(dataSize*3)/reconstructTime.Seconds()/1024/1024)

	ok, err = enc.Verify(testShards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("重建后验证结果: %v\n", ok)

	// 场景3：混合丢失
	fmt.Println("\n3.3 数据和校验分片混合丢失")
	testShards = make([][]byte, len(shards))
	copy(testShards, shards)

	testShards[1] = nil  // 丢失数据分片
	testShards[3] = nil  // 丢失数据分片
	testShards[12] = nil // 丢失校验分片

	start = time.Now()
	err = enc.Reconstruct(testShards)
	if err != nil {
		log.Fatal(err)
	}
	reconstructTime = time.Since(start)
	fmt.Printf("重建完成，用时: %v\n", reconstructTime)
	fmt.Printf("重建速度: %.2f MB/s\n", float64(dataSize*3)/reconstructTime.Seconds()/1024/1024)

	ok, err = enc.Verify(testShards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("重建后验证结果: %v\n", ok)
}

// 测试分片文件持久化和读取
func testFilePersistence(enc rs.ReedSolomon, shards [][]byte, tmpDir string) {
	// 创建分片目录
	shardsDir := filepath.Join(tmpDir, "shards")
	if err := os.MkdirAll(shardsDir, 0755); err != nil {
		log.Fatal(err)
	}

	// 保存所有分片到文件
	for i, shard := range shards {
		if shard != nil {
			fileName := filepath.Join(shardsDir, fmt.Sprintf("shard_%d.dat", i))
			if err := os.WriteFile(fileName, shard, 0644); err != nil {
				log.Fatalf("保存分片 %d 失败: %v", i, err)
			}
		}
	}
	fmt.Println("所有分片已保存到磁盘")

	// 模拟从磁盘读取分片并进行恢复
	fmt.Println("从磁盘读取分片...")

	// 删除部分分片文件模拟丢失
	os.Remove(filepath.Join(shardsDir, "shard_4.dat"))
	os.Remove(filepath.Join(shardsDir, "shard_11.dat"))

	// 从磁盘读取分片
	loadedShards := make([][]byte, len(shards))
	for i := range loadedShards {
		fileName := filepath.Join(shardsDir, fmt.Sprintf("shard_%d.dat", i))
		data, err := os.ReadFile(fileName)
		if err != nil {
			// 文件不存在表示分片丢失
			fmt.Printf("分片 %d 不存在或读取失败\n", i)
			continue
		}
		loadedShards[i] = data
	}

	// 重建丢失的分片
	fmt.Println("重建丢失的分片...")
	if err := enc.Reconstruct(loadedShards); err != nil {
		log.Fatal(err)
	}

	// 验证重建是否成功
	ok, err := enc.Verify(loadedShards)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("从磁盘读取并重建后验证结果: %v\n", ok)

	// 将重建的分片保存回磁盘
	for i, shard := range loadedShards {
		if shard != nil {
			fileName := filepath.Join(shardsDir, fmt.Sprintf("recovered_shard_%d.dat", i))
			if err := os.WriteFile(fileName, shard, 0644); err != nil {
				log.Fatalf("保存恢复的分片 %d 失败: %v", i, err)
			}
		}
	}
	fmt.Println("所有重建的分片已保存到磁盘")
}
