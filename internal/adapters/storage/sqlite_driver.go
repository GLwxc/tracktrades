package storage

/*
#cgo LDFLAGS: -lsqlite3
#include <sqlite3.h>
#include <stdlib.h>

static int bind_text(sqlite3_stmt* stmt, int idx, const char* val) {
        return sqlite3_bind_text(stmt, idx, val, -1, SQLITE_TRANSIENT);
}
*/
import "C"

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"unsafe"
)

func init() {
	sql.Register("sqlite-simple", &sqliteDriver{})
}

type sqliteDriver struct{}

func (d *sqliteDriver) Open(name string) (driver.Conn, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	var db *C.sqlite3
	if rc := C.sqlite3_open(cName, &db); rc != C.SQLITE_OK {
		err := sqliteError(db)
		if db != nil {
			C.sqlite3_close(db)
		}
		return nil, err
	}
	return &sqliteConn{db: db}, nil
}

type sqliteConn struct {
	db *C.sqlite3
}

func (c *sqliteConn) Prepare(query string) (driver.Stmt, error) {
	return c.prepareContext(context.Background(), query)
}

func (c *sqliteConn) PrepareContext(_ context.Context, query string) (driver.Stmt, error) {
	return c.prepareContext(context.Background(), query)
}

func (c *sqliteConn) prepareContext(_ context.Context, query string) (driver.Stmt, error) {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var stmt *C.sqlite3_stmt
	if rc := C.sqlite3_prepare_v2(c.db, cQuery, -1, &stmt, nil); rc != C.SQLITE_OK {
		return nil, sqliteError(c.db)
	}
	return &sqliteStmt{conn: c, stmt: stmt}, nil
}

func (c *sqliteConn) Close() error {
	if c.db == nil {
		return nil
	}
	if rc := C.sqlite3_close(c.db); rc != C.SQLITE_OK {
		return sqliteError(c.db)
	}
	c.db = nil
	return nil
}

func (c *sqliteConn) Begin() (driver.Tx, error) {
	if _, err := c.ExecContext(context.Background(), "BEGIN", nil); err != nil {
		return nil, err
	}
	return &sqliteTx{conn: c}, nil
}

func (c *sqliteConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	stmt, err := c.prepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	s := stmt.(*sqliteStmt)
	if err := s.bind(args); err != nil {
		return nil, err
	}
	rc := C.sqlite3_step(s.stmt)
	if rc != C.SQLITE_DONE {
		return nil, sqliteError(c.db)
	}
	C.sqlite3_reset(s.stmt)
	return sqliteResult{changes: int64(C.sqlite3_changes(c.db))}, nil
}

func (c *sqliteConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	stmt, err := c.prepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	s := stmt.(*sqliteStmt)
	if err := s.bind(args); err != nil {
		stmt.Close()
		return nil, err
	}
	return &sqliteRows{stmt: s}, nil
}

type sqliteStmt struct {
	conn    *sqliteConn
	stmt    *C.sqlite3_stmt
	columns []string
}

func (s *sqliteStmt) Close() error {
	if s.stmt == nil {
		return nil
	}
	rc := C.sqlite3_finalize(s.stmt)
	s.stmt = nil
	if rc != C.SQLITE_OK {
		return sqliteError(s.conn.db)
	}
	return nil
}

func (s *sqliteStmt) NumInput() int {
	return -1
}

func (s *sqliteStmt) Exec(args []driver.Value) (driver.Result, error) {
	named := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	if err := s.bind(named); err != nil {
		return nil, err
	}
	rc := C.sqlite3_step(s.stmt)
	if rc != C.SQLITE_DONE {
		return nil, sqliteError(s.conn.db)
	}
	C.sqlite3_reset(s.stmt)
	return sqliteResult{changes: int64(C.sqlite3_changes(s.conn.db))}, nil
}

func (s *sqliteStmt) Query(args []driver.Value) (driver.Rows, error) {
	named := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		named[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	if err := s.bind(named); err != nil {
		return nil, err
	}
	return &sqliteRows{stmt: s}, nil
}

func (s *sqliteStmt) bind(args []driver.NamedValue) error {
	C.sqlite3_clear_bindings(s.stmt)
	C.sqlite3_reset(s.stmt)
	for _, arg := range args {
		switch v := arg.Value.(type) {
		case string:
			cStr := C.CString(v)
			rc := C.bind_text(s.stmt, C.int(arg.Ordinal), cStr)
			C.free(unsafe.Pointer(cStr))
			if rc != C.SQLITE_OK {
				return sqliteError(s.conn.db)
			}
		case int64:
			if rc := C.sqlite3_bind_int64(s.stmt, C.int(arg.Ordinal), C.sqlite3_int64(v)); rc != C.SQLITE_OK {
				return sqliteError(s.conn.db)
			}
		case float64:
			if rc := C.sqlite3_bind_double(s.stmt, C.int(arg.Ordinal), C.double(v)); rc != C.SQLITE_OK {
				return sqliteError(s.conn.db)
			}
		case nil:
			if rc := C.sqlite3_bind_null(s.stmt, C.int(arg.Ordinal)); rc != C.SQLITE_OK {
				return sqliteError(s.conn.db)
			}
		default:
			return errors.New("unsupported sqlite parameter type")
		}
	}
	return nil
}

type sqliteRows struct {
	stmt   *sqliteStmt
	closed bool
}

func (r *sqliteRows) Columns() []string {
	if r.stmt.columns != nil {
		return r.stmt.columns
	}
	count := int(C.sqlite3_column_count(r.stmt.stmt))
	cols := make([]string, count)
	for i := 0; i < count; i++ {
		cols[i] = C.GoString(C.sqlite3_column_name(r.stmt.stmt, C.int(i)))
	}
	r.stmt.columns = cols
	return cols
}

func (r *sqliteRows) Next(dest []driver.Value) error {
	rc := C.sqlite3_step(r.stmt.stmt)
	switch rc {
	case C.SQLITE_ROW:
		count := int(C.sqlite3_column_count(r.stmt.stmt))
		for i := 0; i < count && i < len(dest); i++ {
			switch C.sqlite3_column_type(r.stmt.stmt, C.int(i)) {
			case C.SQLITE_INTEGER:
				dest[i] = int64(C.sqlite3_column_int64(r.stmt.stmt, C.int(i)))
			case C.SQLITE_FLOAT:
				dest[i] = float64(C.sqlite3_column_double(r.stmt.stmt, C.int(i)))
			case C.SQLITE_TEXT:
				text := (*C.char)(unsafe.Pointer(C.sqlite3_column_text(r.stmt.stmt, C.int(i))))
				dest[i] = C.GoString(text)
			case C.SQLITE_NULL:
				dest[i] = nil
			default:
				return errors.New("unsupported sqlite column type")
			}
		}
		return nil
	case C.SQLITE_DONE:
		return io.EOF
	default:
		return sqliteError(r.stmt.conn.db)
	}
}

func (r *sqliteRows) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	C.sqlite3_reset(r.stmt.stmt)
	return r.stmt.Close()
}

type sqliteTx struct {
	conn *sqliteConn
}

func (t *sqliteTx) Commit() error {
	_, err := t.conn.ExecContext(context.Background(), "COMMIT", nil)
	return err
}

func (t *sqliteTx) Rollback() error {
	_, err := t.conn.ExecContext(context.Background(), "ROLLBACK", nil)
	return err
}

type sqliteResult struct {
	changes int64
}

func (r sqliteResult) LastInsertId() (int64, error) { return 0, errors.New("not supported") }
func (r sqliteResult) RowsAffected() (int64, error) { return r.changes, nil }

func sqliteError(db *C.sqlite3) error {
	if db == nil {
		return errors.New("sqlite error")
	}
	return errors.New(C.GoString(C.sqlite3_errmsg(db)))
}
