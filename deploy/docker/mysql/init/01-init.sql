-- ==========================================
-- KubePolaris 数据库初始化脚本
-- ==========================================
-- 
-- 注意: 所有数据库表结构由 GORM AutoMigrate 自动管理
-- 参见: internal/database/database.go
-- 
-- 默认数据（管理员用户、系统设置等）由应用启动时自动创建
-- 参见: 
--   - createDefaultUser()
--   - createDefaultSystemSettings()
--   - createDefaultPermissions()
--
-- 此文件仅用于预留扩展，如需添加自定义初始化 SQL，请在下方添加
-- ==========================================

-- 设置字符集（确保连接使用 UTF-8）
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

-- 完成
SELECT 'KubePolaris database initialized - tables managed by GORM AutoMigrate' AS message;
