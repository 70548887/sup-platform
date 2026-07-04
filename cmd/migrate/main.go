package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/70548887/sup-platform/migrations"
)

func main() {
	fmt.Println("SUP Platform Migration Tool")

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		log.Fatal("DB_DSN or MYSQL_DSN environment variable is required")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect database failed: %v", err)
	}

	if err := migrations.RunAll(db); err != nil {
		log.Fatalf("run migrations failed: %v", err)
	}

	log.Println("All migrations completed successfully")
}
