package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"sso/internal/domain/models"
	"sso/internal/storage"

	"github.com/mattn/go-sqlite3"
)

type Storage struct {
	db *sql.DB
}

func New(storagePath string) (*Storage, error) {
	const op = "storage.sqlite.New"

	db, err := sql.Open("sqlite3", storagePath+"?_journal_mode=WAL&mode=rwc")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return &Storage{db: db}, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) SaveUser(ctx context.Context, email string, passHash []byte, steamURL string, pathToPhoto string) (int64, error) {
	const op = "storage.sqlite.SaveUser"

	stmt, err := s.db.Prepare("INSERT INTO users(email, pass_hash, steam_url, path_to_photo) VALUES(?, ?, ?, ?);")
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	res, err := stmt.ExecContext(ctx, email, passHash, steamURL, pathToPhoto)
	if err != nil {
		var sqliteErr sqlite3.Error
		if errors.As(err, &sqliteErr) && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique {
			return 0, fmt.Errorf("%s: %w", op, storage.ErrUserExists)
		}
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("%s: %w", op, err)
	}

	return id, nil
}

func (s *Storage) User(ctx context.Context, email string) (models.User, error) {
	const op = "storage.sqlite.User"

	stmt, err := s.db.Prepare("SELECT * FROM users WHERE email = ?;")
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, email)

	var user models.User
	err = row.Scan(&user.ID, &user.Email, &user.PassHash, &user.SteamURL, &user.PathToPhoto, &user.IsAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

func (s *Storage) UserByID(ctx context.Context, id int64) (models.User, error) {
	const op = "storage.sqlite.UserByID"

	stmt, err := s.db.Prepare("SELECT * FROM users WHERE id = ?;")
	if err != nil {
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, id)

	var user models.User
	err = row.Scan(&user.ID, &user.Email, &user.PassHash, &user.SteamURL, &user.PathToPhoto, &user.IsAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}
		return models.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return user, nil
}

func (s *Storage) IsAdmin(ctx context.Context, userID int64) (bool, error) {
	const op = "storage.sqlite.IsAdmin"

	stmt, err := s.db.Prepare("SELECT is_admin FROM users WHERE id = ?;")
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, userID)

	var isAdmin bool
	err = row.Scan(&isAdmin)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, fmt.Errorf("%s: %w", op, storage.ErrUserNotFound)
		}
		return false, fmt.Errorf("%s: %w", op, err)
	}

	return isAdmin, nil
}

func (s *Storage) App(ctx context.Context, appID int32) (models.App, error) {
	const op = "storage.sqlite.App"

	stmt, err := s.db.Prepare("SELECT id, name, secret FROM apps WHERE id = ?;")
	if err != nil {
		return models.App{}, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(ctx, appID)

	var app models.App
	err = row.Scan(&app.ID, &app.Name, &app.Secret)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.App{}, fmt.Errorf("%s: %w", op, storage.ErrAppNotFound)
		}
		return models.App{}, fmt.Errorf("%s: %w", op, err)
	}

	return app, nil
}

func (s *Storage) GetUsers(ctx context.Context) ([]models.User, error) {
	const op = "storage.sqlite.GetUsers"

	stmt, err := s.db.Prepare("SELECT * FROM users;")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer stmt.Close()

	rows, err := stmt.QueryContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err = rows.Scan(&user.ID, &user.Email, &user.PassHash, &user.SteamURL, &user.PathToPhoto, &user.IsAdmin)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}
		users = append(users, user)
	}

	return users, nil
}

type UpdateModel struct {
	ID          int64
	Email       string
	Password    string
	SteamURL    string
	PathToPhoto string
	IsAdmin     bool
}

func (s *Storage) UpdateUser(ctx context.Context, user UpdateModel) error {
	const op = "storage.sqlite.UpdateUser"

	var stmt *sql.Stmt
	var err error
	if user.Password == "" {
		stmt, err = s.db.Prepare("UPDATE users SET email = ?, is_admin = ?, steam_url = ?, path_to_photo = ? WHERE id = ?;")
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		defer stmt.Close()
		_, err = stmt.ExecContext(ctx, user.Email, user.IsAdmin, user.SteamURL, user.PathToPhoto, user.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

	} else if user.Password != "" {
		stmt, err = s.db.Prepare("UPDATE users SET email = ?, is_admin = ?, steam_url = ?, path_to_photo = ?, pass_hash = ? WHERE id = ?;")
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		defer stmt.Close()
		_, err = stmt.ExecContext(ctx, user.Email, user.IsAdmin, user.SteamURL, user.PathToPhoto, user.Password, user.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

	}

	return nil
}
