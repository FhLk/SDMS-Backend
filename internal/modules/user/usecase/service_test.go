package usecase

import (
	"context"
	"errors"
	"testing"

	"sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type userRepositoryStub struct {
	createFn             func(context.Context, *domain.User) error
	findByIDFn           func(context.Context, uuid.UUID) (*domain.User, error)
	findByUsernameFn     func(context.Context, string) (*domain.User, error)
	findByEmployeeCodeFn func(context.Context, string) (*domain.User, error)
	listFn               func(context.Context) ([]domain.User, error)
	updateFn             func(context.Context, *domain.User) error
	deleteFn             func(context.Context, uuid.UUID) error
}

func (s *userRepositoryStub) Create(ctx context.Context, user *domain.User) error {
	if s.createFn != nil {
		return s.createFn(ctx, user)
	}
	return nil
}

func (s *userRepositoryStub) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, domain.ErrUserNotFound
}

func (s *userRepositoryStub) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	if s.findByUsernameFn != nil {
		return s.findByUsernameFn(ctx, username)
	}
	return nil, domain.ErrUserNotFound
}

func (s *userRepositoryStub) FindByEmployeeCode(ctx context.Context, code string) (*domain.User, error) {
	if s.findByEmployeeCodeFn != nil {
		return s.findByEmployeeCodeFn(ctx, code)
	}
	return nil, domain.ErrUserNotFound
}

func (s *userRepositoryStub) List(ctx context.Context) ([]domain.User, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return []domain.User{}, nil
}

func (s *userRepositoryStub) Update(ctx context.Context, user *domain.User) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, user)
	}
	return nil
}

func (s *userRepositoryStub) Delete(ctx context.Context, id uuid.UUID) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func validCreateInput() CreateUserInput {
	return CreateUserInput{
		Username: "somchai", EmployeeCode: "EMP-001", Prefix: "นาย",
		FirstName: "สมชาย", LastName: "ใจดี", Role: domain.RoleTeacher,
	}
}

func validUpdateInput() UpdateUserInput {
	return UpdateUserInput{
		Username: "somchai", EmployeeCode: "EMP-001", Prefix: "นาย",
		FirstName: "สมชาย", LastName: "ใจดี", Role: domain.RoleTeacher,
	}
}

func TestUserServiceCreate(t *testing.T) {
	t.Run("trims, checks uniqueness, and persists", func(t *testing.T) {
		var created *domain.User
		repo := &userRepositoryStub{
			findByUsernameFn: func(_ context.Context, username string) (*domain.User, error) {
				if username != "somchai" {
					t.Errorf("username = %q", username)
				}
				return nil, domain.ErrUserNotFound
			},
			findByEmployeeCodeFn: func(_ context.Context, code string) (*domain.User, error) {
				if code != "EMP-001" {
					t.Errorf("employee code = %q", code)
				}
				return nil, domain.ErrUserNotFound
			},
			createFn: func(_ context.Context, user *domain.User) error {
				created = user
				return nil
			},
		}
		service := NewUserService(repo)
		user, err := service.Create(context.Background(), CreateUserInput{
			Username: "  somchai  ", EmployeeCode: "  EMP-001  ", Prefix: "  นาย  ",
			FirstName: "  สมชาย  ", LastName: "  ใจดี  ", Role: " TEACHER ",
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if user != created || user.UID == uuid.Nil {
			t.Fatal("created user was not generated and persisted")
		}
		if user.Username != "somchai" || user.EmployeeCode != "EMP-001" || user.Prefix != "นาย" ||
			user.FirstName != "สมชาย" || user.LastName != "ใจดี" || user.Role != domain.RoleTeacher ||
			user.Status != domain.StatusActive {
			t.Errorf("user = %+v", user)
		}
	})

	validationTests := []struct {
		name    string
		mutate  func(*CreateUserInput)
		wantErr error
	}{
		{"username required", func(in *CreateUserInput) { in.Username = " " }, domain.ErrUsernameRequired},
		{"employee code required", func(in *CreateUserInput) { in.EmployeeCode = " " }, domain.ErrEmployeeCodeRequired},
		{"prefix required", func(in *CreateUserInput) { in.Prefix = " " }, domain.ErrPrefixRequired},
		{"first name required", func(in *CreateUserInput) { in.FirstName = " " }, domain.ErrFirstNameRequired},
		{"last name required", func(in *CreateUserInput) { in.LastName = " " }, domain.ErrLastNameRequired},
		{"invalid role", func(in *CreateUserInput) { in.Role = "ADMIN" }, domain.ErrInvalidRole},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			repo := &userRepositoryStub{findByUsernameFn: func(context.Context, string) (*domain.User, error) {
				called = true
				return nil, nil
			}}
			input := validCreateInput()
			tt.mutate(&input)
			user, err := NewUserService(repo).Create(context.Background(), input)
			if !errors.Is(err, tt.wantErr) || user != nil || called {
				t.Fatalf("Create() = %+v, %v, repositoryCalled=%v", user, err, called)
			}
		})
	}

	t.Run("duplicate username stops before employee lookup", func(t *testing.T) {
		employeeLookupCalled := false
		repo := &userRepositoryStub{
			findByUsernameFn: func(context.Context, string) (*domain.User, error) { return &domain.User{UID: uuid.New()}, nil },
			findByEmployeeCodeFn: func(context.Context, string) (*domain.User, error) {
				employeeLookupCalled = true
				return nil, nil
			},
		}
		user, err := NewUserService(repo).Create(context.Background(), validCreateInput())
		if !errors.Is(err, domain.ErrUsernameAlreadyExists) || user != nil || employeeLookupCalled {
			t.Fatalf("Create() = %+v, %v, employeeLookupCalled=%v", user, err, employeeLookupCalled)
		}
	})

	t.Run("duplicate employee code", func(t *testing.T) {
		repo := &userRepositoryStub{findByEmployeeCodeFn: func(context.Context, string) (*domain.User, error) {
			return &domain.User{UID: uuid.New()}, nil
		}}
		user, err := NewUserService(repo).Create(context.Background(), validCreateInput())
		if !errors.Is(err, domain.ErrEmployeeCodeAlreadyExists) || user != nil {
			t.Fatalf("Create() = %+v, %v", user, err)
		}
	})

	stages := []struct {
		name string
		repo *userRepositoryStub
	}{
		{"username lookup", &userRepositoryStub{findByUsernameFn: func(context.Context, string) (*domain.User, error) { return nil, errors.New("username lookup") }}},
		{"employee lookup", &userRepositoryStub{findByEmployeeCodeFn: func(context.Context, string) (*domain.User, error) { return nil, errors.New("employee lookup") }}},
		{"create", &userRepositoryStub{createFn: func(context.Context, *domain.User) error { return errors.New("create") }}},
	}
	for _, tt := range stages {
		t.Run("propagates "+tt.name+" error", func(t *testing.T) {
			user, err := NewUserService(tt.repo).Create(context.Background(), validCreateInput())
			if err == nil || user != nil {
				t.Fatalf("Create() = %+v, %v", user, err)
			}
		})
	}
}

func TestUserServiceQueries(t *testing.T) {
	id := uuid.New()
	wantUser := &domain.User{UID: id, Username: "somchai"}
	wantErr := errors.New("query failed")

	t.Run("List success", func(t *testing.T) {
		service := NewUserService(&userRepositoryStub{listFn: func(context.Context) ([]domain.User, error) {
			return []domain.User{*wantUser}, nil
		}})
		users, err := service.List(context.Background())
		if err != nil || len(users) != 1 || users[0] != *wantUser {
			t.Fatalf("List() = %+v, %v", users, err)
		}
	})

	t.Run("List error", func(t *testing.T) {
		service := NewUserService(&userRepositoryStub{listFn: func(context.Context) ([]domain.User, error) { return nil, wantErr }})
		users, err := service.List(context.Background())
		if !errors.Is(err, wantErr) || users != nil {
			t.Fatalf("List() = %+v, %v", users, err)
		}
	})

	t.Run("GetByID rejects nil ID", func(t *testing.T) {
		called := false
		service := NewUserService(&userRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) {
			called = true
			return nil, nil
		}})
		user, err := service.GetByID(context.Background(), uuid.Nil)
		if !errors.Is(err, domain.ErrInvalidUserID) || user != nil || called {
			t.Fatalf("GetByID() = %+v, %v, called=%v", user, err, called)
		}
	})

	t.Run("GetByID success and error", func(t *testing.T) {
		repo := &userRepositoryStub{findByIDFn: func(_ context.Context, gotID uuid.UUID) (*domain.User, error) {
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			return wantUser, nil
		}}
		user, err := NewUserService(repo).GetByID(context.Background(), id)
		if err != nil || user != wantUser {
			t.Fatalf("GetByID() = %+v, %v", user, err)
		}
		repo.findByIDFn = func(context.Context, uuid.UUID) (*domain.User, error) { return nil, wantErr }
		user, err = NewUserService(repo).GetByID(context.Background(), id)
		if !errors.Is(err, wantErr) || user != nil {
			t.Fatalf("GetByID() error case = %+v, %v", user, err)
		}
	})

	t.Run("GetByUsername trims and queries", func(t *testing.T) {
		repo := &userRepositoryStub{findByUsernameFn: func(_ context.Context, username string) (*domain.User, error) {
			if username != "somchai" {
				t.Errorf("username = %q", username)
			}
			return wantUser, nil
		}}
		user, err := NewUserService(repo).GetByUsername(context.Background(), "  somchai  ")
		if err != nil || user != wantUser {
			t.Fatalf("GetByUsername() = %+v, %v", user, err)
		}
	})

	t.Run("GetByUsername rejects blank and propagates errors", func(t *testing.T) {
		service := NewUserService(&userRepositoryStub{})
		user, err := service.GetByUsername(context.Background(), "  ")
		if !errors.Is(err, domain.ErrUsernameRequired) || user != nil {
			t.Fatalf("blank GetByUsername() = %+v, %v", user, err)
		}
		service = NewUserService(&userRepositoryStub{findByUsernameFn: func(context.Context, string) (*domain.User, error) { return nil, wantErr }})
		user, err = service.GetByUsername(context.Background(), "somchai")
		if !errors.Is(err, wantErr) || user != nil {
			t.Fatalf("error GetByUsername() = %+v, %v", user, err)
		}
	})
}

func TestUserServiceUpdate(t *testing.T) {
	id := uuid.New()
	existing := func() *domain.User {
		return &domain.User{
			UID: id, Username: "somchai", EmployeeCode: "EMP-001", Prefix: "นาย",
			FirstName: "สมชาย", LastName: "ใจดี", Role: domain.RoleTeacher, Status: domain.StatusActive,
		}
	}

	t.Run("updates fields without uniqueness lookups when identifiers do not change", func(t *testing.T) {
		current := existing()
		uniquenessCalled := false
		updateCalled := false
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return current, nil },
			findByUsernameFn: func(context.Context, string) (*domain.User, error) { uniquenessCalled = true; return nil, nil },
			findByEmployeeCodeFn: func(context.Context, string) (*domain.User, error) { uniquenessCalled = true; return nil, nil },
			updateFn: func(_ context.Context, user *domain.User) error { updateCalled = user == current; return nil },
		}
		input := validUpdateInput()
		input.Prefix, input.FirstName, input.LastName, input.Role = " ดร. ", " ใหม่ ", " ทดสอบ ", " DIRECTOR "
		user, err := NewUserService(repo).Update(context.Background(), id, input)
		if err != nil || user != current || !updateCalled || uniquenessCalled {
			t.Fatalf("Update() = %+v, %v, updateCalled=%v, uniquenessCalled=%v", user, err, updateCalled, uniquenessCalled)
		}
		if user.Prefix != "ดร." || user.FirstName != "ใหม่" || user.LastName != "ทดสอบ" || user.Role != domain.RoleDirector {
			t.Errorf("user = %+v", user)
		}
	})

	t.Run("checks changed identifiers and allows records owned by current user", func(t *testing.T) {
		current := existing()
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return current, nil },
			findByUsernameFn: func(_ context.Context, username string) (*domain.User, error) {
				if username != "newname" { t.Errorf("username = %q", username) }
				return &domain.User{UID: id}, nil
			},
			findByEmployeeCodeFn: func(_ context.Context, code string) (*domain.User, error) {
				if code != "EMP-002" { t.Errorf("code = %q", code) }
				return nil, domain.ErrUserNotFound
			},
		}
		input := validUpdateInput()
		input.Username, input.EmployeeCode = " newname ", " EMP-002 "
		user, err := NewUserService(repo).Update(context.Background(), id, input)
		if err != nil || user.Username != "newname" || user.EmployeeCode != "EMP-002" {
			t.Fatalf("Update() = %+v, %v", user, err)
		}
	})

	t.Run("rejects nil ID and invalid input before lookup", func(t *testing.T) {
		service := NewUserService(&userRepositoryStub{})
		user, err := service.Update(context.Background(), uuid.Nil, validUpdateInput())
		if !errors.Is(err, domain.ErrInvalidUserID) || user != nil {
			t.Fatalf("nil ID Update() = %+v, %v", user, err)
		}
		input := validUpdateInput()
		input.LastName = " "
		user, err = service.Update(context.Background(), id, input)
		if !errors.Is(err, domain.ErrLastNameRequired) || user != nil {
			t.Fatalf("invalid Update() = %+v, %v", user, err)
		}
	})

	t.Run("rejects changed duplicate username", func(t *testing.T) {
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return existing(), nil },
			findByUsernameFn: func(context.Context, string) (*domain.User, error) { return &domain.User{UID: uuid.New()}, nil },
		}
		input := validUpdateInput()
		input.Username = "other"
		user, err := NewUserService(repo).Update(context.Background(), id, input)
		if !errors.Is(err, domain.ErrUsernameAlreadyExists) || user != nil {
			t.Fatalf("Update() = %+v, %v", user, err)
		}
	})

	t.Run("rejects changed duplicate employee code", func(t *testing.T) {
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return existing(), nil },
			findByEmployeeCodeFn: func(context.Context, string) (*domain.User, error) { return &domain.User{UID: uuid.New()}, nil },
		}
		input := validUpdateInput()
		input.EmployeeCode = "EMP-999"
		user, err := NewUserService(repo).Update(context.Background(), id, input)
		if !errors.Is(err, domain.ErrEmployeeCodeAlreadyExists) || user != nil {
			t.Fatalf("Update() = %+v, %v", user, err)
		}
	})

	errorsToPropagate := []struct {
		name string
		repo *userRepositoryStub
		input UpdateUserInput
	}{
		{"find", &userRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return nil, errors.New("find") }}, validUpdateInput()},
		{"username lookup", &userRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return existing(), nil }, findByUsernameFn: func(context.Context, string) (*domain.User, error) { return nil, errors.New("username") }}, func() UpdateUserInput { in := validUpdateInput(); in.Username = "other"; return in }()},
		{"employee lookup", &userRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return existing(), nil }, findByEmployeeCodeFn: func(context.Context, string) (*domain.User, error) { return nil, errors.New("employee") }}, func() UpdateUserInput { in := validUpdateInput(); in.EmployeeCode = "other"; return in }()},
		{"update", &userRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return existing(), nil }, updateFn: func(context.Context, *domain.User) error { return errors.New("update") }}, validUpdateInput()},
	}
	for _, tt := range errorsToPropagate {
		t.Run("propagates "+tt.name+" error", func(t *testing.T) {
			user, err := NewUserService(tt.repo).Update(context.Background(), id, tt.input)
			if err == nil || user != nil {
				t.Fatalf("Update() = %+v, %v", user, err)
			}
		})
	}
}

func TestUserServiceUpdateStatus(t *testing.T) {
	id := uuid.New()

	t.Run("trims validates and persists", func(t *testing.T) {
		current := &domain.User{UID: id, Status: domain.StatusActive}
		updated := false
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return current, nil },
			updateFn: func(_ context.Context, user *domain.User) error { updated = user == current; return nil },
		}
		user, err := NewUserService(repo).UpdateStatus(context.Background(), id, UpdateUserStatusInput{Status: " INACTIVE "})
		if err != nil || user != current || !updated || user.Status != domain.StatusInactive {
			t.Fatalf("UpdateStatus() = %+v, %v, updated=%v", user, err, updated)
		}
	})

	t.Run("rejects nil ID and invalid status", func(t *testing.T) {
		service := NewUserService(&userRepositoryStub{})
		user, err := service.UpdateStatus(context.Background(), uuid.Nil, UpdateUserStatusInput{Status: domain.StatusActive})
		if !errors.Is(err, domain.ErrInvalidUserID) || user != nil {
			t.Fatalf("nil ID = %+v, %v", user, err)
		}
		user, err = service.UpdateStatus(context.Background(), id, UpdateUserStatusInput{Status: "UNKNOWN"})
		if !errors.Is(err, domain.ErrInvalidStatus) || user != nil {
			t.Fatalf("invalid status = %+v, %v", user, err)
		}
	})

	t.Run("propagates find and update errors", func(t *testing.T) {
		service := NewUserService(&userRepositoryStub{findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return nil, errors.New("find") }})
		user, err := service.UpdateStatus(context.Background(), id, UpdateUserStatusInput{Status: domain.StatusActive})
		if err == nil || user != nil { t.Fatalf("find error = %+v, %v", user, err) }

		service = NewUserService(&userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return &domain.User{UID: id}, nil },
			updateFn: func(context.Context, *domain.User) error { return errors.New("update") },
		})
		user, err = service.UpdateStatus(context.Background(), id, UpdateUserStatusInput{Status: domain.StatusActive})
		if err == nil || user != nil { t.Fatalf("update error = %+v, %v", user, err) }
	})
}

func TestUserServiceDelete(t *testing.T) {
	id := uuid.New()

	t.Run("rejects nil ID", func(t *testing.T) {
		if err := NewUserService(&userRepositoryStub{}).Delete(context.Background(), uuid.Nil); !errors.Is(err, domain.ErrInvalidUserID) {
			t.Fatalf("Delete() error = %v", err)
		}
	})

	t.Run("finds then deletes", func(t *testing.T) {
		deleted := false
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return &domain.User{UID: id}, nil },
			deleteFn: func(_ context.Context, gotID uuid.UUID) error { deleted = gotID == id; return nil },
		}
		if err := NewUserService(repo).Delete(context.Background(), id); err != nil || !deleted {
			t.Fatalf("Delete() error = %v, deleted=%v", err, deleted)
		}
	})

	t.Run("stops on find error", func(t *testing.T) {
		deleted := false
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return nil, domain.ErrUserNotFound },
			deleteFn: func(context.Context, uuid.UUID) error { deleted = true; return nil },
		}
		err := NewUserService(repo).Delete(context.Background(), id)
		if !errors.Is(err, domain.ErrUserNotFound) || deleted {
			t.Fatalf("Delete() error = %v, deleted=%v", err, deleted)
		}
	})

	t.Run("propagates delete error", func(t *testing.T) {
		wantErr := errors.New("delete")
		repo := &userRepositoryStub{
			findByIDFn: func(context.Context, uuid.UUID) (*domain.User, error) { return &domain.User{UID: id}, nil },
			deleteFn: func(context.Context, uuid.UUID) error { return wantErr },
		}
		if err := NewUserService(repo).Delete(context.Background(), id); !errors.Is(err, wantErr) {
			t.Fatalf("Delete() error = %v", err)
		}
	})
}
