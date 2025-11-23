CREATE TABLE IF NOT EXISTS pr_reviewers (
    id SERIAL PRIMARY KEY,
    pr_id VARCHAR(16) NOT NULL,
    reviewer_id VARCHAR(100) NOT NULL,
    FOREIGN KEY (pr_id) REFERENCES pull_request(id) ON DELETE CASCADE,
    FOREIGN KEY (reviewer_id) REFERENCES users(id),
    UNIQUE (pr_id, reviewer_id)
    );