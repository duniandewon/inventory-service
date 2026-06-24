package reference

import "errors"

var (
	ErrUnitNotFound   = errors.New("reference: unit not found")
	ErrUnitNameExists = errors.New("reference: unit name already exists")
	ErrUnitInUse      = errors.New("reference: unit is in use")
	ErrEmptyUnitName  = errors.New("reference: unit name must not be empty")
)

type UnitOfMeasure struct {
	ID   int
	Name string
}
