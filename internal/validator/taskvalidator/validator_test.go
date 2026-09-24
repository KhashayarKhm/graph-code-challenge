package taskvalidator_test

import (
	"errors"
	"strings"
	"testing"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/validator/taskvalidator"
)

func assertFields(t *testing.T, fields map[string]string, err error, wantFields []string) {
	t.Helper()

	if len(wantFields) == 0 {
		if err != nil {
			t.Fatalf("validation error = %v, want nil", err)
		}

		if fields != nil {
			t.Fatalf("field errors = %v, want nil", fields)
		}

		return
	}

	if !errors.Is(err, entity.ErrValidation) {
		t.Fatalf("validation error = %v, want one wrapping ErrValidation", err)
	}

	if len(fields) != len(wantFields) {
		t.Fatalf("field errors = %v, want exactly %v", fields, wantFields)
	}

	for _, name := range wantFields {
		if fields[name] == "" {
			t.Errorf("field errors = %v, missing an entry for %q", fields, name)
		}
	}
}

func TestValidateCreateRequest(t *testing.T) {
	valid := param.CreateTaskRequest{Title: "write the adapter", Description: "pgx", Status: entity.TaskStatusPending, Assignee: "kh"}

	tests := map[string]struct {
		mutate     func(*param.CreateTaskRequest)
		wantFields []string
	}{
		"valid":                {func(*param.CreateTaskRequest) {}, nil},
		"empty status allowed": {func(r *param.CreateTaskRequest) { r.Status = "" }, nil},
		"empty description":    {func(r *param.CreateTaskRequest) { r.Description = "" }, nil},
		"empty assignee":       {func(r *param.CreateTaskRequest) { r.Assignee = "" }, nil},
		"missing title":        {func(r *param.CreateTaskRequest) { r.Title = "" }, []string{"title"}},
		"title too long":       {func(r *param.CreateTaskRequest) { r.Title = strings.Repeat("a", entity.MaxTaskTitleLen+1) }, []string{"title"}},
		"title at max":         {func(r *param.CreateTaskRequest) { r.Title = strings.Repeat("a", entity.MaxTaskTitleLen) }, nil},
		"description too long": {func(r *param.CreateTaskRequest) { r.Description = strings.Repeat("a", entity.MaxTaskDescriptionLen+1) }, []string{"description"}},
		"assignee too long":    {func(r *param.CreateTaskRequest) { r.Assignee = strings.Repeat("a", entity.MaxTaskAssigneeLen+1) }, []string{"assignee"}},
		"unknown status":       {func(r *param.CreateTaskRequest) { r.Status = "archived" }, []string{"status"}},
		"several at once":      {func(r *param.CreateTaskRequest) { r.Title = ""; r.Status = "archived" }, []string{"title", "status"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := valid
			tc.mutate(&req)

			fields, err := taskvalidator.New().ValidateCreateRequest(req)
			assertFields(t, fields, err, tc.wantFields)
		})
	}
}

func TestValidateCreateRequestCountsUnicodeCharacters(t *testing.T) {
	tests := map[string]struct {
		req        param.CreateTaskRequest
		wantFields []string
	}{
		"title at max": {
			req: param.CreateTaskRequest{Title: strings.Repeat("ش", entity.MaxTaskTitleLen)},
		},
		"title over max": {
			req:        param.CreateTaskRequest{Title: strings.Repeat("ش", entity.MaxTaskTitleLen+1)},
			wantFields: []string{"title"},
		},
		"description at max": {
			req: param.CreateTaskRequest{Title: "valid", Description: strings.Repeat("ش", entity.MaxTaskDescriptionLen)},
		},
		"description over max": {
			req:        param.CreateTaskRequest{Title: "valid", Description: strings.Repeat("ش", entity.MaxTaskDescriptionLen+1)},
			wantFields: []string{"description"},
		},
		"assignee at max": {
			req: param.CreateTaskRequest{Title: "valid", Assignee: strings.Repeat("ش", entity.MaxTaskAssigneeLen)},
		},
		"assignee over max": {
			req:        param.CreateTaskRequest{Title: "valid", Assignee: strings.Repeat("ش", entity.MaxTaskAssigneeLen+1)},
			wantFields: []string{"assignee"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fields, err := taskvalidator.New().ValidateCreateRequest(tc.req)
			assertFields(t, fields, err, tc.wantFields)
		})
	}
}

func ptr[T any](value T) *T {
	return &value
}

func TestValidateUpdateRequest(t *testing.T) {
	tests := map[string]struct {
		req        param.UpdateTaskRequest
		wantFields []string
	}{
		"title only":        {param.UpdateTaskRequest{ID: 1, Title: ptr("replace")}, nil},
		"status only":       {param.UpdateTaskRequest{ID: 1, Status: ptr(entity.TaskStatusDone)}, nil},
		"clearing assignee": {param.UpdateTaskRequest{ID: 1, Assignee: ptr("")}, nil},
		"clearing desc":     {param.UpdateTaskRequest{ID: 1, Description: ptr("")}, nil},
		"every field":       {param.UpdateTaskRequest{ID: 1, Title: ptr("t"), Description: ptr("d"), Status: ptr(entity.TaskStatusPending), Assignee: ptr("kh")}, nil},
		"nothing to change": {param.UpdateTaskRequest{ID: 1}, []string{"body"}},
		"zero id":           {param.UpdateTaskRequest{ID: 0, Title: ptr("t")}, []string{"id"}},
		"negative id":       {param.UpdateTaskRequest{ID: -3, Title: ptr("t")}, []string{"id"}},
		"blank title":       {param.UpdateTaskRequest{ID: 1, Title: ptr("")}, []string{"title"}},
		"title too long":    {param.UpdateTaskRequest{ID: 1, Title: ptr(strings.Repeat("a", entity.MaxTaskTitleLen+1))}, []string{"title"}},
		"assignee too long": {param.UpdateTaskRequest{ID: 1, Assignee: ptr(strings.Repeat("a", entity.MaxTaskAssigneeLen+1))}, []string{"assignee"}},
		"blank status":      {param.UpdateTaskRequest{ID: 1, Status: ptr(entity.TaskStatus(""))}, []string{"status"}},
		"unknown status":    {param.UpdateTaskRequest{ID: 1, Status: ptr(entity.TaskStatus("archived"))}, []string{"status"}},
		"bad id and title":  {param.UpdateTaskRequest{ID: 0, Title: ptr("")}, []string{"id", "title"}},
		"empty body wins":   {param.UpdateTaskRequest{ID: 0}, []string{"id", "body"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fields, err := taskvalidator.New().ValidateUpdateRequest(tc.req)
			assertFields(t, fields, err, tc.wantFields)
		})
	}
}

func TestValidateUpdateDistinguishesOmittedFromCleared(t *testing.T) {
	validator := taskvalidator.New()

	if _, err := validator.ValidateUpdateRequest(param.UpdateTaskRequest{ID: 1, Description: ptr("")}); err != nil {
		t.Errorf("clearing description = %v, want nil: an explicit empty string is a real value", err)
	}

	fields, err := validator.ValidateUpdateRequest(param.UpdateTaskRequest{ID: 1})
	if err == nil {
		t.Fatal("omitting every field = nil, want an error")
	}

	if fields["body"] != taskvalidator.MsgUpdateEmpty {
		t.Errorf("body message = %q, want %q", fields["body"], taskvalidator.MsgUpdateEmpty)
	}
}

func TestValidateListRequest(t *testing.T) {
	tests := map[string]struct {
		req        param.ListTasksRequest
		wantFields []string
	}{
		"empty is valid":     {param.ListTasksRequest{}, nil},
		"all filters set":    {param.ListTasksRequest{Status: entity.TaskStatusDone, Assignee: "kh", Cursor: 10, Limit: 5}, nil},
		"unknown status":     {param.ListTasksRequest{Status: "archived"}, []string{"status"}},
		"assignee too long":  {param.ListTasksRequest{Assignee: strings.Repeat("a", entity.MaxTaskAssigneeLen+1)}, []string{"assignee"}},
		"negative cursor":    {param.ListTasksRequest{Cursor: -1}, []string{"cursor"}},
		"huge limit is fine": {param.ListTasksRequest{Limit: 10_000}, nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fields, err := taskvalidator.New().ValidateListRequest(tc.req)
			assertFields(t, fields, err, tc.wantFields)
		})
	}
}

func TestStatusMessageListsEveryStatus(t *testing.T) {
	fields, _ := taskvalidator.New().ValidateListRequest(param.ListTasksRequest{Status: "archived"})

	for _, status := range entity.TaskStatusList() {
		if !strings.Contains(fields["status"], string(status)) {
			t.Errorf("status message %q does not mention %q", fields["status"], status)
		}
	}
}
