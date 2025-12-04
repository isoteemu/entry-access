package storage

import "time"

type Entry struct {
	ID          int64      `db:"id"`
	Name        string     `db:"name"`
	CalendarURL string     `db:"calendar_url,omitempty"`
	CreatedAt   time.Time  `db:"created_at"`
	DeletedAt   *time.Time `db:"deleted_at,omitempty"`
}
