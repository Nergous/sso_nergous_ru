ALTER TABLE users
    DROP COLUMN lockout_until,
    DROP COLUMN failed_login_attempts;