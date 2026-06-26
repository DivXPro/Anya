-- Add Kimi as an API-based agent option.
INSERT OR IGNORE INTO agents (id, name, command, enabled) VALUES
    ('kimi', 'Kimi', 'kimi-api', 0);

-- Default model for Kimi API. Users must set kimi_api_key in settings.
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('kimi_model', 'moonshot-v1-8k');
