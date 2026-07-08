package errors

import "errors"

const (
	CodeFunctionNumInvalid           = "function_num_invalid"
	CodeFunctionIconInvalid          = "function_icon_invalid"
	CodeFunctionNameRequired         = "function_name_required"
	CodeFunctionNumTaken             = "function_num_taken"
	CodeFunctionNotFound             = "function_not_found"
	CodeOnlyOwnerCanEdit             = "only_owner_can_edit"
	CodeTemplateNotOwned             = "template_not_owned"
	CodeFunctionReplaceSourceInvalid = "function_replace_source_invalid"
	CodeFunctionDurationInvalid      = "function_duration_invalid"
)

var (
	ErrFunctionNumInvalid           = errors.New(CodeFunctionNumInvalid)
	ErrFunctionIconInvalid          = errors.New(CodeFunctionIconInvalid)
	ErrFunctionNameRequired         = errors.New(CodeFunctionNameRequired)
	ErrFunctionNumTaken             = errors.New(CodeFunctionNumTaken)
	ErrFunctionNotFound             = errors.New(CodeFunctionNotFound)
	ErrOnlyOwnerCanEdit             = errors.New(CodeOnlyOwnerCanEdit)
	ErrTemplateNotOwned             = errors.New(CodeTemplateNotOwned)
	ErrFunctionReplaceSourceInvalid = errors.New(CodeFunctionReplaceSourceInvalid)
	ErrFunctionDurationInvalid      = errors.New(CodeFunctionDurationInvalid)
)
