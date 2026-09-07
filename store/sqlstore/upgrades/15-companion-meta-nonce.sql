-- v15 (compatible with v8+): Add companion meta nonce column to device table
ALTER TABLE pakaiwa_device ADD COLUMN companion_meta_nonce TEXT NOT NULL DEFAULT '';
