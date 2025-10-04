package main

import (
	"context"
	"fmt"
	"log"

	"kubepolaris/internal/models"
	"kubepolaris/internal/services"
)

func main() {
	// 创建 Prometheus 服务
	prometheusService := services.NewPrometheusService()

	// 测试配置
	config := &models.MonitoringConfig{
		Type:     "prometheus",
		Endpoint: "http://localhost:9090", // 假设本地有 Prometheus
		Auth: &models.MonitoringAuth{
			Type:     "basic",
			Username: "admin",
			Password: "admin",
		},
		Labels: map[string]string{
			"cluster": "test-cluster",
		},
	}

	ctx := context.Background()

	// 测试连接
	fmt.Println("测试 Prometheus 连接...")
	if err := prometheusService.TestConnection(ctx, config); err != nil {
		log.Printf("连接测试失败: %v", err)
		fmt.Println("注意：如果 Prometheus 未运行，这是正常的")
	} else {
		fmt.Println("连接测试成功！")
	}

	// 测试查询集群指标
	fmt.Println("\n测试查询集群指标...")
	metrics, err := prometheusService.QueryClusterMetrics(ctx, config, "test-cluster", "1h", "1m")
	if err != nil {
		log.Printf("查询集群指标失败: %v", err)
		fmt.Println("注意：如果 Prometheus 未运行或没有数据，这是正常的")
	} else {
		fmt.Printf("查询成功！获取到指标数据:\n")
		if metrics.CPU != nil {
			fmt.Printf("  CPU 使用率: %.2f%%\n", metrics.CPU.Current)
		}
		if metrics.Memory != nil {
			fmt.Printf("  内存使用率: %.2f%%\n", metrics.Memory.Current)
		}
		if metrics.Pods != nil {
			fmt.Printf("  Pod 总数: %d, 运行中: %d\n", metrics.Pods.Total, metrics.Pods.Running)
		}
	}

	// 测试 VictoriaMetrics 配置
	fmt.Println("\n测试 VictoriaMetrics 配置...")
	vmConfig := &models.MonitoringConfig{
		Type:     "victoriametrics",
		Endpoint: "http://localhost:8428", // 假设本地有 VictoriaMetrics
		Auth: &models.MonitoringAuth{
			Type:  "bearer",
			Token: "test-token",
		},
		Labels: map[string]string{
			"cluster":     "test-cluster",
			"environment": "test",
		},
	}

	if err := prometheusService.TestConnection(ctx, vmConfig); err != nil {
		log.Printf("VictoriaMetrics 连接测试失败: %v", err)
		fmt.Println("注意：如果 VictoriaMetrics 未运行，这是正常的")
	} else {
		fmt.Println("VictoriaMetrics 连接测试成功！")
	}

	fmt.Println("\n测试完成！")
	fmt.Println("\n使用说明：")
	fmt.Println("1. 确保 Prometheus 或 VictoriaMetrics 服务正在运行")
	fmt.Println("2. 在 KubePolaris 界面中配置正确的监控端点")
	fmt.Println("3. 测试连接确保配置正确")
	fmt.Println("4. 查看集群/节点/Pod 的监控图表")
}
