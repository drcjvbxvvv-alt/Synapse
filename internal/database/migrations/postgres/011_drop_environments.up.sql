-- 011: 移除 Environment 抽象層
-- Pipeline Run 直接帶 cluster_id + namespace，不再透過 Environment 間接解析

-- 移除 environment_id FK 欄位（若存在）
ALTER TABLE pipeline_runs DROP COLUMN IF EXISTS environment_id;

DROP TABLE IF EXISTS promotion_history;
DROP TABLE IF EXISTS environments;
