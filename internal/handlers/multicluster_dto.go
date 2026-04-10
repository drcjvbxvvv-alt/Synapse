package handlers

// MigrateRequest 遷移請求
type MigrateRequest struct {
	SourceClusterID uint   `json:"sourceClusterId" binding:"required"`
	SourceNamespace string `json:"sourceNamespace" binding:"required"`
	WorkloadKind    string `json:"workloadKind" binding:"required"` // Deployment / StatefulSet / DaemonSet
	WorkloadName    string `json:"workloadName" binding:"required"`
	TargetClusterID uint   `json:"targetClusterId" binding:"required"`
	TargetNamespace string `json:"targetNamespace" binding:"required"`
	SyncConfigMaps  bool   `json:"syncConfigMaps"`
	SyncSecrets     bool   `json:"syncSecrets"`
}

// MigrateCheckRequest 遷移預檢請求（同 MigrateRequest）
type MigrateCheckRequest = MigrateRequest

// MigrateCheckResult 預檢結果
type MigrateCheckResult struct {
	Feasible       bool    `json:"feasible"`
	Message        string  `json:"message"`
	WorkloadCPUReq float64 `json:"workloadCpuReq"` // millicores
	WorkloadMemReq float64 `json:"workloadMemReq"` // MiB
	TargetFreeCPU  float64 `json:"targetFreeCpu"`  // millicores
	TargetFreeMem  float64 `json:"targetFreeMem"`  // MiB
	ConfigMapCount int     `json:"configMapCount"`
	SecretCount    int     `json:"secretCount"`
}

// MigrateResult 遷移結果
type MigrateResult struct {
	Success          bool     `json:"success"`
	WorkloadCreated  bool     `json:"workloadCreated"`
	ConfigMapsSynced []string `json:"configMapsSynced"`
	SecretsSynced    []string `json:"secretsSynced"`
	Message          string   `json:"message"`
}

// syncDetailItem 同步詳情項目
type syncDetailItem struct {
	ClusterID uint   `json:"clusterId"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}
