package models

type ErrorResponse struct {
	Message string `json:"message"`
}

type BadRequestErrorResponse struct {
	Message string                  `json:"message"`
	Details []ValidationErrorDetail `json:"details"`
}

type ValidationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Type    string `json:"type"`
}
