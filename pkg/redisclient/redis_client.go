package redisclient

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8" // 修复：确保仅导入一次redis包，无重复声明
)

// RedisConfig Redis连接配置（可根据项目扩展）
type RedisConfig struct {
	UnixSocketPath string        // Unix Socket路径
	TCPAddr        string        // 默认TCP地址（修复：修正注释拼写错误）
	Password       string        // Redis密码（无则空字符串）
	DB             int           // 数据库编号
	RetryTimes     int           // Unix Socket重试次数
	RetryInterval  time.Duration // 重试间隔
}

// DefaultRedisConfig 默认配置（导出供全项目使用，并非未使用）
var DefaultRedisConfig = RedisConfig{
	UnixSocketPath: "/var/run/redis/redis-server.sock", // 默认Unix Socket路径
	TCPAddr:        "127.0.0.1:6379",                   // 默认TCP地址
	Password:       "",                                 // 默认无密码
	DB:             0,                                  // 默认DB 0
	RetryTimes:     3,                                  // 重试3次
	RetryInterval:  500 * time.Millisecond,             // 每次重试间隔500ms
}

// NewRedisClient 创建Redis客户端（优先Unix Socket，失败降级TCP）
// 导出供全项目调用，并非未使用
func NewRedisClient(cfg RedisConfig) (*redis.Client, error) {
	// 1. 尝试Unix Socket连接（重试指定次数）
	var unixClient *redis.Client
	var unixErr error

	for i := 1; i <= cfg.RetryTimes; i++ {
		unixClient = redis.NewClient(&redis.Options{
			Network:  "unix",             // 指定Unix Socket协议（修复：修正注释拼写）
			Addr:     cfg.UnixSocketPath, // Unix Socket路径
			Password: cfg.Password,       // 密码
			DB:       cfg.DB,             // 数据库编号
		})

		// 修复：循环内defer会导致cancel延迟到函数结束，改用局部作用域确保及时释放
		func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel() // 局部作用域内defer，每次循环都会执行cancel，避免资源泄漏

			// 测试连接（ping）
			_, unixErr = unixClient.Ping(ctx).Result()
		}()

		if unixErr == nil {
			fmt.Printf("✅ Unix Socket连接Redis成功（重试次数：%d）\n", i)
			return unixClient, nil
		}

		// 修复：重试失败时关闭当前unixClient，避免资源泄漏
		_ = unixClient.Close()

		// 未成功则打印错误，等待重试
		fmt.Printf("❌ Unix Socket连接Redis失败（重试次数：%d），错误：%v\n", i, unixErr)
		if i < cfg.RetryTimes {
			time.Sleep(cfg.RetryInterval)
		}
	}

	// 2. Unix Socket重试失败，降级为TCP连接
	fmt.Println("⚠️ Unix Socket连接全部失败，降级为TCP连接Redis")
	tcpClient := redis.NewClient(&redis.Options{
		Network:  "tcp",        // TCP协议
		Addr:     cfg.TCPAddr,  // TCP地址
		Password: cfg.Password, // 密码
		DB:       cfg.DB,       // 数据库编号
	})

	// 测试TCP连接（修复：确保ctx及时释放）
	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := tcpClient.Ping(ctx).Err(); err != nil {
			_ = tcpClient.Close() // 连接失败时关闭TCP客户端，避免泄漏
			unixErr = fmt.Errorf("TCP连接Redis也失败：%v", err)
		}
	}()

	if unixErr != nil {
		return nil, unixErr
	}

	fmt.Println("✅ TCP连接Redis成功")
	return tcpClient, nil
}
