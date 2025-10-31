package storage

import "time"

type Entry struct {
	ID        int64      `db:"id"`
	Name      string     `db:"name"`
	CreatedAt time.Time  `db:"created_at"`
	DeletedAt *time.Time `db:"deleted_at,omitempty"`
}
