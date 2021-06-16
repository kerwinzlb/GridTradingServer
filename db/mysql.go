package db

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Mysql struct {
	endpoint string
	db       *sql.DB
}

func NewMysql(endpoint string) *Mysql {
	return &Mysql{
		endpoint: endpoint,
	}
}

func (m *Mysql) Connect() error {
	db, err := sql.Open("mysql", m.endpoint)
	if err != nil {
		return err
	}
	err = db.Ping()
	if err != nil {
		return err
	}
	m.db = db

	return nil
}

func (m *Mysql) DisConnect() error {
	m.db.Close()
	return nil
}

func closeStmt(stmt *sql.Stmt) {
	if stmt != nil {
		stmt.Close()
	}
}

func (m *Mysql) Execute(cmd string) (sql.Result, error) {
	err := m.db.Ping()
	if err != nil {
		err = m.Connect()
		if err != nil {
			return nil, err
		}
	}
	stmt, err := m.db.Prepare(cmd)
	defer closeStmt(stmt)

	if err != nil {
		return nil, err
	}
	return stmt.Exec()
}

func (m *Mysql) QueryRow(cmd string, dest ...interface{}) error {
	err := m.db.Ping()
	if err != nil {
		err = m.Connect()
		if err != nil {
			return err
		}
	}
	return m.db.QueryRow(cmd).Scan(dest...)
}
