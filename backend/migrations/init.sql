-- SecureStream Database Schema
-- KVKK/GDPR uyumlu güvenlik log izleme platformu

-- UUID extension
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- -------------------------------------------------------
-- TENANTS (Müşteriler / Organizasyonlar)
-- -------------------------------------------------------
CREATE TABLE IF NOT EXISTS tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    email       VARCHAR(255) NOT NULL UNIQUE,
    api_key     VARCHAR(64)  NOT NULL UNIQUE, -- SHA-256 hash
    plan        VARCHAR(50)  NOT NULL DEFAULT 'free', -- free, pro, enterprise
    is_active   BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- -------------------------------------------------------
-- LOG ENTRIES (Gelen ham loglar)
-- -------------------------------------------------------
CREATE TABLE IF NOT EXISTS log_entries (
    id          BIGSERIAL    PRIMARY KEY,
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source      VARCHAR(100) NOT NULL, -- sshd, postgresql, firewall, custom
    raw_log     TEXT         NOT NULL,
    source_ip   INET,
    username    VARCHAR(255),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Performans için index
CREATE INDEX IF NOT EXISTS idx_log_entries_tenant ON log_entries(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_log_entries_ip ON log_entries(source_ip);

-- -------------------------------------------------------
-- ALERTS (Tespit edilen tehditler - Audit Trail)
-- -------------------------------------------------------
CREATE TABLE IF NOT EXISTS alerts (
    id          BIGSERIAL    PRIMARY KEY,
    tenant_id   UUID         NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    alert_type  VARCHAR(100) NOT NULL, -- brute_force, unauthorized_access, port_scan, db_breach
    severity    VARCHAR(20)  NOT NULL, -- critical, high, medium, low
    source_ip   INET,
    username    VARCHAR(255),
    message     TEXT         NOT NULL,
    raw_log     TEXT,
    acknowledged BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alerts_tenant ON alerts(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alerts_severity ON alerts(tenant_id, severity);
CREATE INDEX IF NOT EXISTS idx_alerts_type ON alerts(tenant_id, alert_type);

-- -------------------------------------------------------
-- DEMO TENANT (Geliştirme için)
-- API Key: "dev-api-key-12345" -> SHA256 hash'i aşağıda
-- -------------------------------------------------------
INSERT INTO tenants (name, email, api_key, plan)
VALUES (
    'Demo Şirketi A.Ş.',
    'demo@securestream.dev',
    -- echo -n "dev-api-key-12345" | sha256sum
    '8264dc9f07e749d9c2ffead0b25de8cb22bed7af774e189ef224ae015908776b',
    'pro'
) ON CONFLICT DO NOTHING;
