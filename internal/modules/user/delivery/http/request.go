package http

type CreateUserRequest struct {
	Username     string `json:"username"`
	EmployeeCode string `json:"employee_code"`
	Prefix       string `json:"prefix"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Role         string `json:"role"`
}

type UpdateUserRequest struct {
	Username     string `json:"username"`
	EmployeeCode string `json:"employee_code"`
	Prefix       string `json:"prefix"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Role         string `json:"role"`
}

type UpdateUserStatusRequest struct {
	Status string `json:"status"`
}
