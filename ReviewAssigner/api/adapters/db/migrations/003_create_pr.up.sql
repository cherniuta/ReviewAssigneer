CREATE TABLE IF NOT EXISTS pull_request (
    id VARCHAR(16) NOT NULL PRIMARY KEY,
    title VARCHAR(160) NOT NULL,
    author_id VARCHAR(100) NOT NULL,
    state VARCHAR(20) DEFAULT 'OPEN' CHECK (state IN ('OPEN', 'MERGED', 'CLOSED')),
    FOREIGN KEY (author_id) REFERENCES users(id)
    );