-- +goose Up
CREATE TABLE articles(
    id SERIAL PRIMARY KEY,
    source_id INT NOT NULL,
    link VARCHAR(255)NOT NULL, 
    published_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL,
    posted_at TIMESTAMP, 
    CONSTRAINT fk_articles_source_id
        FOREIGN KEY (source_id)
        REFERENCES sources(id)
        ON DELETE CASCADE   
        ON UPDATE RESTRICT  
);

-- +goose Down
DROP TABLE IF EXISTS articles;
