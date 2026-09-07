-- v12 (compatible with v8+): Add sender timestamp and prune index for privacy tokens
ALTER TABLE pakaiwa_privacy_tokens ADD COLUMN sender_timestamp BIGINT;

CREATE INDEX idx_pakaiwa_privacy_tokens_our_jid_timestamp
ON pakaiwa_privacy_tokens (our_jid, timestamp);
