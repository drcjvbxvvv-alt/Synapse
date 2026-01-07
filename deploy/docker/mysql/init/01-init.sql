-- ==========================================
-- KubePolaris 数据库初始化脚本
-- ==========================================

-- 设置字符集
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

-- 创建数据库（如果不存在）
CREATE DATABASE IF NOT EXISTS kubepolaris 
    DEFAULT CHARACTER SET utf8mb4 
    DEFAULT COLLATE utf8mb4_unicode_ci;

USE kubepolaris;

-- ==========================================
-- 用户表
-- ==========================================
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(50) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    email VARCHAR(100),
    phone VARCHAR(20),
    role VARCHAR(20) NOT NULL DEFAULT 'viewer',
    status TINYINT NOT NULL DEFAULT 1,
    last_login_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    PRIMARY KEY (id),
    INDEX idx_username (username),
    INDEX idx_email (email),
    INDEX idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ==========================================
-- 集群表
-- ==========================================
CREATE TABLE IF NOT EXISTS clusters (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(200),
    description TEXT,
    api_server VARCHAR(500) NOT NULL,
    kubeconfig LONGTEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'unknown',
    version VARCHAR(50),
    node_count INT DEFAULT 0,
    pod_count INT DEFAULT 0,
    last_check_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    PRIMARY KEY (id),
    INDEX idx_name (name),
    INDEX idx_status (status),
    INDEX idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ==========================================
-- 操作日志表
-- ==========================================
CREATE TABLE IF NOT EXISTS operation_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED,
    username VARCHAR(50),
    cluster_id BIGINT UNSIGNED,
    cluster_name VARCHAR(100),
    resource_type VARCHAR(50),
    resource_name VARCHAR(200),
    namespace VARCHAR(100),
    operation VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'success',
    detail TEXT,
    request_ip VARCHAR(50),
    user_agent VARCHAR(500),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_user_id (user_id),
    INDEX idx_cluster_id (cluster_id),
    INDEX idx_resource_type (resource_type),
    INDEX idx_operation (operation),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ==========================================
-- 权限表
-- ==========================================
CREATE TABLE IF NOT EXISTS permissions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    cluster_id BIGINT UNSIGNED,
    namespace VARCHAR(100),
    role VARCHAR(50) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    INDEX idx_user_id (user_id),
    INDEX idx_cluster_id (cluster_id),
    UNIQUE KEY uk_user_cluster_ns (user_id, cluster_id, namespace)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ==========================================
-- 系统设置表
-- ==========================================
CREATE TABLE IF NOT EXISTS system_settings (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    category VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    value TEXT,
    description VARCHAR(500),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_category_name (category, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ==========================================
-- 插入默认管理员用户
-- 密码: admin123 (BCrypt 加密)
-- ==========================================
INSERT INTO users (username, password, email, role, status) 
VALUES ('admin', '$2a$10$N.zmdr9k7uOCQb376NoUnuTJ8iAt6Z5EHsM8lE9lBOsl7iAt6Z5EH', 'admin@kubepolaris.io', 'admin', 1)
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;

-- ==========================================
-- 插入默认系统设置
-- ==========================================
INSERT INTO system_settings (category, name, value, description) VALUES
    ('general', 'site_name', 'KubePolaris', '站点名称'),
    ('general', 'site_description', 'Kubernetes Multi-Cluster Management Platform', '站点描述'),
    ('auth', 'session_timeout', '24', '会话超时时间（小时）'),
    ('auth', 'max_login_attempts', '5', '最大登录尝试次数'),
    ('monitoring', 'metrics_retention', '7', '监控数据保留天数'),
    ('audit', 'log_retention', '90', '审计日志保留天数')
ON DUPLICATE KEY UPDATE updated_at = CURRENT_TIMESTAMP;

-- 完成
SELECT 'KubePolaris database initialized successfully!' AS message;

