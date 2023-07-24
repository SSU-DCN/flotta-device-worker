CREATE TABLE IF NOT EXISTS device (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    manufacturer TEXT NULL,
    model TEXT NULL,
    sw_version TEXT NULL,
    identifiers TEXT NULL,
    protocol TEXT NULL,
    connection TEXT NULL,
    battery TEXT NULL
);
