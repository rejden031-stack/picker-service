-- Роли пользователей: добавляем admin и флаг блокировки.
ALTER TABLE users ADD COLUMN blocked BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('worker', 'senior', 'admin'));

-- Журнал аудита: кто, что, когда.
CREATE TABLE audit_events (
    id         BIGSERIAL PRIMARY KEY,
    ts         TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_id   BIGINT NOT NULL DEFAULT 0,
    actor_name TEXT   NOT NULL DEFAULT '',
    action     TEXT   NOT NULL,
    entity_id  BIGINT NOT NULL DEFAULT 0,
    details    TEXT   NOT NULL DEFAULT ''
);

CREATE INDEX audit_events_ts_idx ON audit_events (ts DESC);