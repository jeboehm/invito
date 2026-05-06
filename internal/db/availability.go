package db

import "database/sql"

type AvailabilityRule struct {
	ID        int64
	UserID    int64
	Weekday   int
	StartTime string
	EndTime   string
	Active    bool
}

func ListAvailabilityRules(db *sql.DB, userID int64) ([]AvailabilityRule, error) {
	rows, err := db.Query(`
		SELECT id, user_id, weekday, start_time, end_time, active
		FROM availability_rules WHERE user_id = ? ORDER BY weekday
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []AvailabilityRule
	for rows.Next() {
		var r AvailabilityRule
		if err := rows.Scan(&r.ID, &r.UserID, &r.Weekday, &r.StartTime, &r.EndTime, &r.Active); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func ReplaceAvailabilityRules(db *sql.DB, userID int64, rules []AvailabilityRule) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM availability_rules WHERE user_id = ?`, userID); err != nil {
		return err
	}

	for _, r := range rules {
		active := 0
		if r.Active {
			active = 1
		}
		if _, err := tx.Exec(`
			INSERT INTO availability_rules (user_id, weekday, start_time, end_time, active)
			VALUES (?, ?, ?, ?, ?)
		`, userID, r.Weekday, r.StartTime, r.EndTime, active); err != nil {
			return err
		}
	}

	return tx.Commit()
}
