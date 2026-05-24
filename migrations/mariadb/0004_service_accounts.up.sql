CREATE TABLE IF NOT EXISTS service_accounts (
    id                      CHAR(36)            NOT NULL,
    name                    VARCHAR(128)        NOT NULL,
    description             VARCHAR(1024)       NOT NULL,
    client_secret_hash      VARBINARY(255)      NOT NULL,
    status                  TINYINT UNSIGNED    NOT NULL,
    etag                    CHAR(36)            NOT NULL,
    created_at              DATETIME(6)         NOT NULL,
    updated_at              DATETIME(6)         NOT NULL,
    last_authenticated_at   DATETIME(6)         NULL,

    PRIMARY KEY (id),
    UNIQUE KEY uk_service_accounts_name (name),
    KEY idx_service_accounts_status_created (status, created_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
