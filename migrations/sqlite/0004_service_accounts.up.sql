CREATE TABLE IF NOT EXISTS service_accounts (
    id                      TEXT        NOT NULL, 
    name                    TEXT        NOT NULL, 
    description             TEXT        NOT NULL, 
    client_secret_hash      BLOB        NOT NULL, 
    status                  INTEGER     NOT NULL, 
    etag                    TEXT        NOT NULL, 
    created_at              TEXT        NOT NULL, 
    updated_at              TEXT        NOT NULL, 
    last_authenticated_at   TEXT        NULL,     

    PRIMARY KEY (id),
    CONSTRAINT uk_service_accounts_name UNIQUE (name)
);

CREATE INDEX idx_service_accounts_status_created 
ON service_accounts(status, created_at, id);