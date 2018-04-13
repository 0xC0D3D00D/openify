package sql

import (
	"database/sql"

	doorservicepb "github.com/0xc0d3d00d/openify/go/proto/doorservice"
	_ "github.com/lib/pq"
)

type Sql struct {
	db *sql.DB
}

func New() (*Sql, error) {
	sqlInstance := &Sql{}
	db, err := sql.Open("postgres", "postgres://openify@localhost:26257/openify?sslmode=disable")
	if err != nil {
		return nil, err
	}
	sqlInstance.db = db
	return sqlInstance, nil
}

type AccessLog struct {
	DoorId int64
	State  doorservicepb.DoorState
	UserId *int64
}

func (sql *Sql) StoreAccessLog(accessLog AccessLog) error {
	tx, err := sql.db.Begin()
	if err != nil {
		return err
	}

	_, err = tx.Exec("INSERT INTO accesslog (door_id, state, user_id, created_at) VALUES ($1, $2, $3, now())",
		accessLog.DoorId, accessLog.State, accessLog.UserId)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}
