package storage

import "time"

type Entry struct {
	ID          int64      `db:"id"`
	Name        string     `db:"name"`
	CalendarURL string     `db:"calendar_url,omitempty"`
	CreatedAt   time.Time  `db:"created_at"`
	DeletedAt   *time.Time `db:"deleted_at,omitempty"`
}

type DeviceStatus string

const (
	DeviceStatusPending  DeviceStatus = "pending"
	DeviceStatusApproved DeviceStatus = "approved"
	DeviceStatusRejected DeviceStatus = "rejected"
)

type Device struct {
	DeviceID   string       `db:"device_id"`
	ClientIP   string       `db:"client_ip"`
	CreatedAt  time.Time    `db:"created_at"`
	UpdatedAt  time.Time    `db:"updated_at"`
	Status     DeviceStatus `db:"status"`
	ApprovedBy *string      `db:"approved_by"`
}

type ApprovedDevice struct {
	ID         int64      `db:"id"`
	DeviceID   string     `db:"device_id"`
	EntryID    int64      `db:"entry_id"`
	ApprovedBy string     `db:"approved_by"`
	ApprovedAt time.Time  `db:"approved_at"`
	RevokedAt  *time.Time `db:"revoked_at"`
}
