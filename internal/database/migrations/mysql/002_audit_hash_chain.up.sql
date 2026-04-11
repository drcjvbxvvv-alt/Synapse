-- P2-2: Add hash-chain columns to audit_logs
-- prev_hash: SHA-256 of the previous record (zeroHash for the first record)
-- hash:      SHA-256 over (prev_hash, user_id, action, resource_type,
--                          resource_ref, result, ip, created_at.UnixNano)
--
-- DEFAULT '' allows existing rows to be back-filled lazily; VerifyChain
-- automatically skips records whose hash is empty.

ALTER TABLE `audit_logs`
  ADD COLUMN `prev_hash` varchar(64) NOT NULL DEFAULT '' AFTER `details`,
  ADD COLUMN `hash`      varchar(64) NOT NULL DEFAULT '' AFTER `prev_hash`,
  ADD INDEX  `idx_audit_logs_hash` (`hash`);
