package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"sdms/internal/config"
	userdomain "sdms/internal/modules/user/domain"
	userpostgres "sdms/internal/modules/user/repository/postgres"
	userusecase "sdms/internal/modules/user/usecase"
	"sdms/internal/platform/database"

	"github.com/joho/godotenv"
)

func main() {
	username := flag.String("username", "", "admin username")
	password := flag.String("password", "", "admin password (minimum 8 characters)")
	employeeCode := flag.String("employee-code", "", "employee code")
	prefix := flag.String("prefix", "", "name prefix")
	firstName := flag.String("first-name", "", "first name")
	lastName := flag.String("last-name", "", "last name")
	flag.Parse()

	if *username == "" || *password == "" || *employeeCode == "" || *prefix == "" || *firstName == "" || *lastName == "" {
		flag.Usage()
		log.Fatal("all seed-admin flags are required")
	}

	_ = godotenv.Load()
	cfg := config.Load()
	db, err := database.NewPostgres(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}

	repo := userpostgres.NewUserRepository(db)
	service := userusecase.NewUserService(repo)
	user, err := service.Create(context.Background(), userusecase.CreateUserInput{
		Username:     *username,
		EmployeeCode: *employeeCode,
		Prefix:       *prefix,
		FirstName:    *firstName,
		LastName:     *lastName,
		Role:         userdomain.RoleAdmin,
		Password:     *password,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("admin created: %s (%s)\n", user.Username, user.UID)
}
