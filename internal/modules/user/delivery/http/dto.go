package http

type CreateUserRequest struct {
	Username     string `json:"email"`
	EmployeeCode string `json:"employee_code"`
	Prefix       string `json:"prefix"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Role         string `json:"role"`
}

type UserResponse struct {
	UID          uint   `json:"uid"`
	Username     string `json:"username"`
	EmployeeCode string `json:"employee_code"`
	Prefix       string `json:"prefix"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Role         string `json:"role"`
	Status       string `json:"status"`
}
