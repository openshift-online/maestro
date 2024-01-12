package services

import (
	e "errors"
	"strings"

	"github.com/bwmarrin/snowflake"
	"gorm.io/gorm"

	"github.com/openshift-online/maestro/pkg/errors"
)

// Field names suspected to contain personally identifiable information
var piiFields []string = []string{
	"username",
	"first_name",
	"last_name",
	"email",
	"address",
}

func handleGetError(resourceType, field string, value interface{}, err error) *errors.ServiceError {
	// Sanitize errors of any personally identifiable information
	for _, f := range piiFields {
		if field == f {
			value = "<redacted>"
			break
		}
	}
	if e.Is(err, gorm.ErrRecordNotFound) {
		return errors.NotFound("%s with %s='%v' not found", resourceType, field, value)
	}
	return errors.GeneralError("Unable to find %s with %s='%v': %s", resourceType, field, value, err)
}

func handleCreateError(resourceType string, err error) *errors.ServiceError {
	if strings.Contains(err.Error(), "violates unique constraint") {
		return errors.Conflict("This %s already exists", resourceType)
	}
	return errors.GeneralError("Unable to create %s: %s", resourceType, err.Error())
}

func handleUpdateError(resourceType string, err error) *errors.ServiceError {
	if strings.Contains(err.Error(), "violates unique constraint") {
		return errors.Conflict("Changes to %s conflict with existing records", resourceType)
	}
	return errors.GeneralError("Unable to update %s: %s", resourceType, err.Error())
}

func handleDeleteError(resourceType string, err error) *errors.ServiceError {
	return errors.GeneralError("Unable to delete %s: %s", resourceType, err.Error())
}

// compareSequenceIDs compares two snowflake sequence IDs and returns true if the first ID is greater than the second.
func compareSequenceIDs(sequenceID1, sequenceID2 string) (bool, error) {
	// If the second sequence ID is empty, then the first is greater
	if sequenceID1 != "" && sequenceID2 == "" {
		return true, nil
	}
	id1, err := snowflake.ParseString(sequenceID1)
	if err != nil {
		return false, errors.GeneralError("Unable to parse sequence ID: %s", err.Error())
	}
	id2, err := snowflake.ParseString(sequenceID2)
	if err != nil {
		return false, errors.GeneralError("Unable to parse sequence ID: %s", err.Error())
	}

	if id1.Node() != id2.Node() {
		return false, errors.GeneralError("Sequence IDs are not from the same node")
	}

	if id1.Time() != id2.Time() {
		return id1.Time() > id2.Time(), nil
	}

	return id1.Step() > id2.Step(), nil
}
