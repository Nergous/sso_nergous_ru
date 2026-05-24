package passwordhash

import "golang.org/x/crypto/bcrypt"

func Hash(plaintext string, cost int) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(plaintext), cost)
}

func Compare(hash []byte, plaintext string) error {
	return bcrypt.CompareHashAndPassword(hash, []byte(plaintext))
}
