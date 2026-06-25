package logistics

import (
	"database/sql"

	"github.com/duniandewon/inventory-service/internal/inventory"
)

type Repository struct {
	db      *sql.DB
	invRepo *inventory.Repository
}

func NewRepository(db *sql.DB, invRepo *inventory.Repository) *Repository {
	return &Repository{db: db, invRepo: invRepo}
}

func nullInt(v int) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}
