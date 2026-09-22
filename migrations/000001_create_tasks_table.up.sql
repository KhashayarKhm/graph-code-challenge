CREATE TABLE tasks (
    id          BIGSERIAL     PRIMARY KEY,
    title       VARCHAR(200)  NOT NULL,
    description VARCHAR(2000) NOT NULL DEFAULT '',
    status      VARCHAR(50)   NOT NULL,
    assignee    VARCHAR(100)  NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_tasks_live_id ON tasks (id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_status_id ON tasks (status, id) WHERE deleted_at IS NULL;
CREATE INDEX idx_tasks_assignee_id ON tasks (assignee, id) WHERE deleted_at IS NULL;
