-- Initial schema
CREATE TABLE IF NOT EXISTS sessions (
	session_id              TEXT PRIMARY KEY,
	project_name            TEXT,
	first_timestamp         TEXT,
	last_timestamp          TEXT,
	git_branch              TEXT,
	total_input_tokens      INTEGER DEFAULT 0,
	total_output_tokens     INTEGER DEFAULT 0,
	total_cache_read        INTEGER DEFAULT 0,
	total_cache_creation    INTEGER DEFAULT 0,
	model                   TEXT,
	turn_count              INTEGER DEFAULT 0,
	tool                    TEXT NOT NULL DEFAULT 'claude_code'
);

CREATE TABLE IF NOT EXISTS turns (
	id                      INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id              TEXT,
	timestamp               TEXT,
	model                   TEXT,
	input_tokens            INTEGER DEFAULT 0,
	output_tokens           INTEGER DEFAULT 0,
	cache_read_tokens       INTEGER DEFAULT 0,
	cache_creation_tokens   INTEGER DEFAULT 0,
	tool_name               TEXT,
	cwd                     TEXT,
	tool                    TEXT NOT NULL DEFAULT 'claude_code'
);

CREATE TABLE IF NOT EXISTS processed_files (
	path    TEXT PRIMARY KEY,
	mtime   REAL,
	lines   INTEGER
);

CREATE INDEX IF NOT EXISTS idx_turns_session    ON turns(session_id);
CREATE INDEX IF NOT EXISTS idx_turns_timestamp  ON turns(timestamp);
CREATE INDEX IF NOT EXISTS idx_sessions_first   ON sessions(first_timestamp);
