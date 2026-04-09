package staffing

import (
	"testing"

	"github.com/jacksonlee411/Bugs-And-Blossoms/modules/staffing/domain/ports"
)

func TestNewAssignmentsFacade(t *testing.T) {
	var store ports.AssignmentStore
	_ = NewAssignmentsFacade(store)
}
