CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(100) NOT NULL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    team_name VARCHAR(100) NOT NULL,
    active BOOLEAN DEFAULT true,
    FOREIGN KEY (team_name) REFERENCES teams(name)
    );