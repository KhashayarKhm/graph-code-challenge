package taskvalidator

import (
	"fmt"
	"strings"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
)

const (
	MsgIDInvalid       = "id must be a positive number"
	MsgTitleRequired   = "title is required"
	MsgTitleTooLong    = "title must not exceed %d characters"
	MsgDescTooLong     = "description must not exceed %d characters"
	MsgAssigneeTooLong = "assignee must not exceed %d characters"
	MsgStatusRequired  = "status is required"
	MsgStatusInvalid   = "status must be one of: %s"
	MsgCursorNegative  = "cursor must not be negative"
)

type Validator struct{}

func New() Validator {
	return Validator{}
}

func checkID(id int64) string {
	if id < 1 {
		return MsgIDInvalid
	}

	return ""
}

func checkTitle(title string) string {
	switch {
	case title == "":
		return MsgTitleRequired
	case len(title) > entity.MaxTaskTitleLen:
		return fmt.Sprintf(MsgTitleTooLong, entity.MaxTaskTitleLen)
	}

	return ""
}

func checkDescription(description string) string {
	if len(description) > entity.MaxTaskDescriptionLen {
		return fmt.Sprintf(MsgDescTooLong, entity.MaxTaskDescriptionLen)
	}

	return ""
}

func checkAssignee(assignee string) string {
	if len(assignee) > entity.MaxTaskAssigneeLen {
		return fmt.Sprintf(MsgAssigneeTooLong, entity.MaxTaskAssigneeLen)
	}

	return ""
}

func checkStatus(status entity.TaskStatus, required bool) string {
	if status == "" {
		if required {
			return MsgStatusRequired
		}

		return ""
	}

	if !status.IsValid() {
		return fmt.Sprintf(MsgStatusInvalid, statusNames())
	}

	return ""
}

func checkCursor(cursor int64) string {
	if cursor < 0 {
		return MsgCursorNegative
	}

	return ""
}

func statusNames() string {
	statuses := entity.TaskStatusList()
	names := make([]string, 0, len(statuses))

	for _, status := range statuses {
		names = append(names, string(status))
	}

	return strings.Join(names, ", ")
}

func set(fields map[string]string, name, message string) {
	if message != "" {
		fields[name] = message
	}
}

func result(fields map[string]string) (map[string]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}

	return fields, fmt.Errorf("taskvalidator: invalid request: %w", entity.ErrValidation)
}

func (v Validator) ValidateCreateRequest(req param.CreateTaskRequest) (map[string]string, error) {
	fields := make(map[string]string)

	set(fields, "title", checkTitle(req.Title))
	set(fields, "description", checkDescription(req.Description))
	set(fields, "assignee", checkAssignee(req.Assignee))
	set(fields, "status", checkStatus(req.Status, false))

	return result(fields)
}

func (v Validator) ValidateUpdateRequest(req param.UpdateTaskRequest) (map[string]string, error) {
	fields := make(map[string]string)

	set(fields, "id", checkID(req.ID))
	set(fields, "title", checkTitle(req.Title))
	set(fields, "description", checkDescription(req.Description))
	set(fields, "assignee", checkAssignee(req.Assignee))
	set(fields, "status", checkStatus(req.Status, true))

	return result(fields)
}

func (v Validator) ValidateListRequest(req param.ListTasksRequest) (map[string]string, error) {
	fields := make(map[string]string)

	set(fields, "assignee", checkAssignee(req.Assignee))
	set(fields, "status", checkStatus(req.Status, false))
	set(fields, "cursor", checkCursor(req.Cursor))

	return result(fields)
}
