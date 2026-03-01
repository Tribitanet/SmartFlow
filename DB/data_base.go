package database

import (
	"fmt"
	"os"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	db *gorm.DB
)

func Init(dsn string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}

func GetDB() *gorm.DB {
	return db
}

func CheckDB() error {
	dsn := GetDSN()
	fmt.Println(dsn)
	db, err := Init(dsn)
	if err != nil {
		return err
	}

	dbInst, err := db.DB()
	if err != nil {
		return err
	}

	return dbInst.Ping()
}

func GetDSN() string {
	user := os.Getenv("PG_USER")
	if user == "" {
		user = "smartflow"
	}
	pass := os.Getenv("PG_PASS")
	if pass == "" {
		pass = "12345678"
	}
	return fmt.Sprintf("postgres://%s:%s@127.0.0.1:5432/%s?sslmode=disable", user, pass, user)
}

