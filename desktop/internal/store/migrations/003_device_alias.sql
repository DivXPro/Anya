ALTER TABLE authorized_devices ADD COLUMN alias TEXT;
UPDATE authorized_devices SET alias = name WHERE alias IS NULL OR alias = '';
