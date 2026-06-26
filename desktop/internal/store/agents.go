package store

import "database/sql"

func ListAgents(db *sql.DB) ([]Agent, error) {
	rows, err := db.Query("SELECT id, name, command, enabled, version, config FROM agents ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Name, &a.Command, &a.Enabled, &a.Version, &a.Config); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, nil
}

func GetAgent(db *sql.DB, id string) (*Agent, error) {
	a := &Agent{}
	err := db.QueryRow(
		"SELECT id, name, command, enabled, version, config FROM agents WHERE id = ?", id,
	).Scan(&a.ID, &a.Name, &a.Command, &a.Enabled, &a.Version, &a.Config)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func UpdateAgent(db *sql.DB, a *Agent) error {
	_, err := db.Exec(
		"UPDATE agents SET name=?, command=?, enabled=?, version=?, config=? WHERE id=?",
		a.Name, a.Command, a.Enabled, a.Version, a.Config, a.ID,
	)
	return err
}

func InsertAgent(db *sql.DB, a *Agent) error {
	_, err := db.Exec(
		"INSERT INTO agents (id, name, command, enabled, version, config) VALUES (?, ?, ?, ?, ?, ?)",
		a.ID, a.Name, a.Command, a.Enabled, a.Version, a.Config,
	)
	return err
}

func UpdateAgentEnabled(db *sql.DB, id string, enabled bool) error {
	_, err := db.Exec("UPDATE agents SET enabled = ? WHERE id = ?", boolToInt(enabled), id)
	return err
}

func UpdateAgentVersion(db *sql.DB, id string, version string) error {
	_, err := db.Exec("UPDATE agents SET version = ? WHERE id = ?", version, id)
	return err
}

func DisableAllAgents(db *sql.DB) error {
	_, err := db.Exec("UPDATE agents SET enabled = 0")
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
