CREATE TABLE IF NOT EXISTS slos (
    id                  bigserial        PRIMARY KEY,
    cluster_id          bigint           NOT NULL,
    name                varchar(255)     NOT NULL,
    description         varchar(1024)    NOT NULL DEFAULT '',
    namespace           varchar(128)     NOT NULL DEFAULT '',
    sli_type            varchar(32)      NOT NULL,
    prom_query          text             NOT NULL,
    total_query         text,
    target              double precision NOT NULL,
    window              varchar(16)      NOT NULL,
    burn_rate_warning   double precision NOT NULL DEFAULT 2,
    burn_rate_critical  double precision NOT NULL DEFAULT 10,
    enabled             boolean          NOT NULL DEFAULT true,
    created_at          timestamptz(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          timestamptz(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at          timestamptz(3)
);
CREATE INDEX IF NOT EXISTS idx_slos_cluster_id ON slos (cluster_id);
CREATE INDEX IF NOT EXISTS idx_slos_namespace ON slos (namespace);
CREATE INDEX IF NOT EXISTS idx_slos_deleted_at ON slos (deleted_at);
