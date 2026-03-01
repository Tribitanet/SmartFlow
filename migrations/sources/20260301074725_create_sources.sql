-- +goose Up
CREATE TABLE sources(
    id SERIAL PRIMARY KEY,
    s_name VARCHAR(255) NOT NULL,
    feed_link VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS sources;
