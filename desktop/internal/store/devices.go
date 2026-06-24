package store

import "database/sql"

type AuthorizedDevice struct {
	DeviceID     string `json:"device_id"`
	Name         string `json:"name"`
	AuthorizedAt string `json:"authorized_at"`
	LastSeenIP   string `json:"last_seen_ip"`
	LastSeenAt   string `json:"last_seen_at"`
	Revoked      bool   `json:"revoked"`
}

func IsDeviceAuthorized(db *sql.DB, deviceID string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(*) FROM authorized_devices WHERE device_id = ? AND revoked = 0",
		deviceID,
	).Scan(&count)
	return count > 0, err
}

func AuthorizeDevice(db *sql.DB, deviceID, name string) error {
	_, err := db.Exec(
		`INSERT INTO authorized_devices (device_id, name) VALUES (?, ?)
		 ON CONFLICT(device_id) DO UPDATE SET name = excluded.name, revoked = 0, authorized_at = datetime('now')`,
		deviceID, name,
	)
	return err
}

func RevokeDevice(db *sql.DB, deviceID string) error {
	_, err := db.Exec(
		"UPDATE authorized_devices SET revoked = 1 WHERE device_id = ?",
		deviceID,
	)
	return err
}

func UpdateDeviceLastSeen(db *sql.DB, deviceID, ip string) error {
	_, err := db.Exec(
		"UPDATE authorized_devices SET last_seen_ip = ?, last_seen_at = datetime('now') WHERE device_id = ?",
		ip, deviceID,
	)
	return err
}

func ListAuthorizedDevices(db *sql.DB) ([]AuthorizedDevice, error) {
	rows, err := db.Query(
		"SELECT device_id, name, authorized_at, COALESCE(last_seen_ip, ''), COALESCE(last_seen_at, ''), revoked FROM authorized_devices ORDER BY authorized_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []AuthorizedDevice
	for rows.Next() {
		var d AuthorizedDevice
		if err := rows.Scan(&d.DeviceID, &d.Name, &d.AuthorizedAt, &d.LastSeenIP, &d.LastSeenAt, &d.Revoked); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, nil
}
